// Command worker-autoscaler is the external poller that publishes Temporal
// task-queue backlog metrics to CloudWatch so Application Auto Scaling can
// resize the ECS worker service. It is the CREMA-pattern equivalent of the
// KEDA Temporal scaler used in Kubernetes deployments.
//
// The binary runs as a Lambda (provided.al2023) invoked every 30 s by an
// EventBridge Scheduler. It calls DescribeTaskQueue with ReportStats=true
// for both workflow and activity task queue types on the ECS worker queue,
// sums the approximate backlog counts, takes the max backlog age, and
// publishes both as gauges under the "TemporalDemo/Worker" CloudWatch
// namespace.
//
// Failure mode: on any error (Temporal unreachable, TLS issue, CloudWatch
// throttle) the handler logs and returns nil. The next scheduled invocation
// retries — there is no backoff or in-process retry. Missing metric data
// causes Auto Scaling to hold capacity, which is the safe default.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/alexandreroman/aws-image-processing-demo/internal/temporalclient"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	enumspb "go.temporal.io/api/enums/v1"
	taskqueuepb "go.temporal.io/api/taskqueue/v1"
	"go.temporal.io/api/workflowservice/v1"
	"google.golang.org/grpc"
)

const (
	metricNamespace      = "TemporalDemo/Worker"
	metricBacklogCount   = "BacklogCount"
	metricBacklogAgeSecs = "BacklogAgeSeconds"
	describeTimeout      = 8 * time.Second
)

// taskQueueTypes is the canonical pair we aggregate over. Workflow tasks
// drive scheduling, activity tasks drive throughput; both must be summed
// because a worker that polls only one type still pulls from the same ECS
// service.
var taskQueueTypes = []enumspb.TaskQueueType{
	enumspb.TASK_QUEUE_TYPE_WORKFLOW,
	enumspb.TASK_QUEUE_TYPE_ACTIVITY,
}

// queueDescriber is the narrow slice of the Temporal workflow service used
// by aggregateBacklog. Extracted as an interface so the aggregation logic
// can be unit-tested without a live Temporal connection.
type queueDescriber interface {
	DescribeTaskQueue(
		ctx context.Context,
		req *workflowservice.DescribeTaskQueueRequest,
	) (*workflowservice.DescribeTaskQueueResponse, error)
}

type backlog struct {
	count int64
	age   time.Duration
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	lambda.Start(handler(logger))
}

// handler returns a Lambda handler closure. Returning nil on every code
// path is deliberate: Lambda retries on error would compound load when
// Temporal is already unhealthy, and the 30 s scheduler tick is the natural
// retry cadence.
func handler(logger *slog.Logger) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if err := run(ctx, logger); err != nil {
			logger.Error("autoscaler poll failed", "err", err)
		}
		return nil
	}
}

func run(ctx context.Context, logger *slog.Logger) error {
	taskQueue := os.Getenv("TEMPORAL_TASK_QUEUE")
	if taskQueue == "" {
		return errors.New("TEMPORAL_TASK_QUEUE is required")
	}

	tc, namespace, err := temporalclient.Dial(logger)
	if err != nil {
		return fmt.Errorf("dial Temporal: %w", err)
	}
	defer tc.Close()

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("load AWS config: %w", err)
	}
	cw := cloudwatch.NewFromConfig(awsCfg)

	describeCtx, cancel := context.WithTimeout(ctx, describeTimeout)
	defer cancel()

	svc := workflowServiceAdapter{tc.WorkflowService()}
	b, err := aggregateBacklog(describeCtx, svc, namespace, taskQueue)
	if err != nil {
		return fmt.Errorf("aggregate backlog: %w", err)
	}

	logger.Info("backlog polled",
		"taskQueue", taskQueue,
		"namespace", namespace,
		"backlogCount", b.count,
		"backlogAgeSeconds", b.age.Seconds(),
	)

	return putMetrics(ctx, cw, taskQueue, b)
}

// aggregateBacklog fans out one DescribeTaskQueue call per task queue type
// in parallel and combines the results: sum of approximate_backlog_count,
// max of approximate_backlog_age. The ECS worker is unversioned (the
// long-running worker.New path does not opt into Worker Versioning), so we
// leave Versions unset and the server returns the aggregated unversioned
// queue stats.
func aggregateBacklog(
	ctx context.Context,
	svc queueDescriber,
	namespace, taskQueue string,
) (backlog, error) {
	results := make([]backlog, len(taskQueueTypes))
	errs := make([]error, len(taskQueueTypes))

	var wg sync.WaitGroup
	for i, tqt := range taskQueueTypes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := svc.DescribeTaskQueue(ctx, &workflowservice.DescribeTaskQueueRequest{
				Namespace:     namespace,
				TaskQueue:     &taskqueuepb.TaskQueue{Name: taskQueue, Kind: enumspb.TASK_QUEUE_KIND_NORMAL},
				TaskQueueType: tqt,
				ReportStats:   true,
			})
			if err != nil {
				errs[i] = err
				return
			}
			results[i] = statsToBacklog(resp.GetStats())
		}()
	}
	wg.Wait()

	if err := errors.Join(errs...); err != nil {
		return backlog{}, err
	}

	var agg backlog
	for _, r := range results {
		agg.count += r.count
		if r.age > agg.age {
			agg.age = r.age
		}
	}
	return agg, nil
}

// statsToBacklog extracts the two numbers we care about, tolerating a nil
// Stats (which the server returns when a task queue type has no recorded
// activity).
func statsToBacklog(s *taskqueuepb.TaskQueueStats) backlog {
	if s == nil {
		return backlog{}
	}
	var age time.Duration
	if d := s.GetApproximateBacklogAge(); d != nil {
		age = d.AsDuration()
	}
	return backlog{count: s.GetApproximateBacklogCount(), age: age}
}

func putMetrics(
	ctx context.Context,
	cw *cloudwatch.Client,
	taskQueue string,
	b backlog,
) error {
	now := time.Now()
	dims := []cwtypes.Dimension{
		{Name: aws.String("TaskQueue"), Value: aws.String(taskQueue)},
	}
	_, err := cw.PutMetricData(ctx, &cloudwatch.PutMetricDataInput{
		Namespace: aws.String(metricNamespace),
		MetricData: []cwtypes.MetricDatum{
			{
				MetricName: aws.String(metricBacklogCount),
				Dimensions: dims,
				Timestamp:  aws.Time(now),
				Unit:       cwtypes.StandardUnitCount,
				Value:      aws.Float64(float64(b.count)),
			},
			{
				MetricName: aws.String(metricBacklogAgeSecs),
				Dimensions: dims,
				Timestamp:  aws.Time(now),
				Unit:       cwtypes.StandardUnitSeconds,
				Value:      aws.Float64(b.age.Seconds()),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("PutMetricData: %w", err)
	}
	return nil
}

// workflowServiceAdapter narrows the Temporal gRPC client (whose
// DescribeTaskQueue takes a variadic grpc.CallOption) to the
// queueDescriber interface used by aggregateBacklog. Keeping the
// adapter trivial means the unit-tested code path is the same as the
// production one.
type workflowServiceAdapter struct {
	svc interface {
		DescribeTaskQueue(
			ctx context.Context,
			req *workflowservice.DescribeTaskQueueRequest,
			opts ...grpc.CallOption,
		) (*workflowservice.DescribeTaskQueueResponse, error)
	}
}

func (a workflowServiceAdapter) DescribeTaskQueue(
	ctx context.Context,
	req *workflowservice.DescribeTaskQueueRequest,
) (*workflowservice.DescribeTaskQueueResponse, error) {
	return a.svc.DescribeTaskQueue(ctx, req)
}
