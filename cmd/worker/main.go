// Command worker is the Temporal worker. A single binary that runs in two
// modes: a long-running worker (host / Docker / ECS Fargate) and an AWS
// Lambda worker. The mode is selected at runtime by the presence of
// AWS_LAMBDA_FUNCTION_NAME, which the Lambda runtime always sets.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/alexandreroman/aws-image-processing-demo/internal/activities"
	"github.com/alexandreroman/aws-image-processing-demo/internal/anthropicclient"
	"github.com/alexandreroman/aws-image-processing-demo/internal/awsclient"
	"github.com/alexandreroman/aws-image-processing-demo/internal/temporalclient"
	"github.com/alexandreroman/aws-image-processing-demo/internal/workflows"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.temporal.io/sdk/contrib/aws/lambdaworker"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

const (
	defaultTaskQueue      = "image-processing-ecs"
	defaultDeploymentName = "image-processing"
)

// buildID identifies this worker's deployment version. Injected at Lambda
// build time via -ldflags "-X main.buildID=...".
var buildID = "dev"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// AWS_LAMBDA_FUNCTION_NAME is set unconditionally by the Lambda runtime;
	// its presence is the canonical signal that we are inside a Lambda
	// execution environment.
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		runLambda(logger)
		return
	}

	if err := runLongRunning(logger); err != nil {
		logger.Error("worker exited with error", "err", err)
		os.Exit(1)
	}
}

func runLongRunning(logger *slog.Logger) error {
	ctx := context.Background()

	s3c, ddb, anth, err := loadAWSClients(ctx)
	if err != nil {
		return err
	}

	tc, namespace, err := temporalclient.Dial(logger)
	if err != nil {
		return err
	}
	defer tc.Close()

	acts, err := activities.New(s3c, ddb, anth, tc)
	if err != nil {
		return err
	}

	// Split on commas so a single dev process can poll both runtime queues
	// at once (e.g. TEMPORAL_TASK_QUEUE=image-processing-ecs,image-processing-lambda).
	queues := splitQueues(envOr("TEMPORAL_TASK_QUEUE", defaultTaskQueue))
	if len(queues) == 0 {
		return errors.New("worker: TEMPORAL_TASK_QUEUE must list at least one queue")
	}

	// Cap concurrent activities so a large burst cannot exhaust the
	// worker's memory (each Resize holds a decoded RGBA buffer). 8 is
	// safe for the 256 MiB compose container; prod (1+ GiB Fargate)
	// overrides via WORKER_MAX_CONCURRENT_ACTIVITIES.
	maxConcurrent := envIntOr("WORKER_MAX_CONCURRENT_ACTIVITIES", 8)
	workers := make([]worker.Worker, 0, len(queues))
	for _, q := range queues {
		w := worker.New(tc, q, worker.Options{
			MaxConcurrentActivityExecutionSize: maxConcurrent,
		})
		registerAll(w, acts)
		if err := w.Start(); err != nil {
			for _, prev := range workers {
				prev.Stop()
			}
			return err
		}
		workers = append(workers, w)
	}

	logger.Info("worker starting",
		"taskQueues", queues,
		"namespace", namespace,
		"bucket", acts.ImagesBucket,
		"table", acts.ImagesTable,
		"maxConcurrentActivities", maxConcurrent,
	)

	// The worker speaks to Temporal over gRPC; this HTTP listener exists
	// purely as a liveness probe for the container orchestrator (compose
	// healthcheck, ECS container healthCheck).
	healthAddr := envOr("HEALTH_ADDR", ":8001")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	srv := &http.Server{
		Addr:              healthAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		logger.Info("health server listening", "addr", healthAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("health server failed", "err", err)
		}
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	// worker.InterruptCh closes on SIGINT/SIGTERM; stop every worker in
	// reverse start order so the last-registered queue drains first.
	<-worker.InterruptCh()
	logger.Info("worker stopping", "taskQueues", queues)
	for i := len(workers) - 1; i >= 0; i-- {
		workers[i].Stop()
	}
	return nil
}

// splitQueues parses a comma-separated list of task queue names, trimming
// whitespace and dropping empties. Returns an empty slice when the input
// has no usable entries.
func splitQueues(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if q := strings.TrimSpace(p); q != "" {
			out = append(out, q)
		}
	}
	return out
}

func runLambda(logger *slog.Logger) {
	ctx := context.Background()

	s3c, ddb, anth, err := loadAWSClients(ctx)
	if err != nil {
		logger.Error("worker: load AWS clients", "err", err)
		os.Exit(1)
	}

	// The lambdaworker manages its own per-invocation Temporal connection for
	// the worker itself. The Activities struct still needs an independent,
	// process-scoped client so StartProcessImage can schedule top-level child
	// workflows from inside an activity. The Lambda execution environment is
	// process-stable across warm invocations, so this client is reused.
	tc, namespace, err := temporalclient.Dial(logger)
	if err != nil {
		logger.Error("worker: dial Temporal for activities", "err", err)
		os.Exit(1)
	}

	// Lambda runs exactly one worker per function, so collapse to the first
	// entry when TEMPORAL_TASK_QUEUE was set to a comma-separated list (as
	// the long-running path accepts).
	queues := splitQueues(envOr("TEMPORAL_TASK_QUEUE", defaultTaskQueue))
	if len(queues) == 0 {
		tc.Close()
		logger.Error("worker: TEMPORAL_TASK_QUEUE must list at least one queue")
		os.Exit(1)
	}
	taskQueue := queues[0]
	if len(queues) > 1 {
		logger.Warn("multiple task queues set in Lambda mode; using the first",
			"selected", taskQueue, "ignored", queues[1:])
	}
	acts, err := activities.New(s3c, ddb, anth, tc)
	if err != nil {
		tc.Close()
		logger.Error("worker: build activities", "err", err)
		os.Exit(1)
	}

	logger.Info("lambda worker starting",
		"taskQueue", taskQueue,
		"namespace", namespace,
		"bucket", acts.ImagesBucket,
		"table", acts.ImagesTable,
		"deploymentName", envOr("WORKER_DEPLOYMENT_NAME", defaultDeploymentName),
		"buildID", buildID,
	)

	version := worker.WorkerDeploymentVersion{
		DeploymentName: envOr("WORKER_DEPLOYMENT_NAME", defaultDeploymentName),
		BuildID:        buildID,
	}

	lambdaworker.RunWorker(version, func(ctx *lambdaworker.Options) error {
		// Use our env-based options (Temporal Cloud mTLS, address, namespace,
		// structured logger) instead of the envconfig defaults the lambdaworker
		// pre-populates.
		opts, _, err := temporalclient.Options(logger)
		if err != nil {
			return err
		}
		ctx.ClientOptions = opts
		ctx.TaskQueue = taskQueue
		ctx.WorkerOptions.MaxConcurrentActivityExecutionSize = envIntOr("WORKER_MAX_CONCURRENT_ACTIVITIES", 2)

		registerAll(ctx, acts)

		ctx.OnShutdown(func(context.Context) error {
			tc.Close()
			return nil
		})
		return nil
	})
}

// registrar is the subset of worker.Registry needed to register this worker's
// workflows and activities. Both *worker.Worker (from worker.New) and
// *lambdaworker.Options satisfy it.
type registrar interface {
	RegisterWorkflowWithOptions(w any, options workflow.RegisterOptions)
	RegisterActivity(a any)
}

// registerAll registers the worker's workflows and activities. VersioningBehaviorPinned
// is set unconditionally: it is a no-op when UseVersioning is false (long-running path)
// and required when lambdaworker forces UseVersioning on.
func registerAll(r registrar, acts *activities.Activities) {
	pinned := workflow.RegisterOptions{VersioningBehavior: workflow.VersioningBehaviorPinned}
	r.RegisterWorkflowWithOptions(workflows.ProcessImage, pinned)
	r.RegisterWorkflowWithOptions(workflows.LaunchPipelines, pinned)
	r.RegisterActivity(acts)
}

func loadAWSClients(ctx context.Context) (*s3.Client, *dynamodb.Client, *anthropicclient.Client, error) {
	awsCfg, err := awsclient.Load(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	s3c := awsclient.NewS3(awsCfg)
	ddb := awsclient.NewDynamoDB(awsCfg)

	anth, err := anthropicclient.New()
	if err != nil {
		return nil, nil, nil, err
	}
	return s3c, ddb, anth, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

// Compile-time check that *lambdaworker.Options satisfies registrar.
// worker.Worker is itself an interface whose method set is a superset of
// registrar, so no static check is needed for it.
var _ registrar = (*lambdaworker.Options)(nil)
