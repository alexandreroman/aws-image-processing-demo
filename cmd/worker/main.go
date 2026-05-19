// Command worker is the Temporal worker. A single binary supports four
// deployment modes that collapse to two code paths: a long-running worker
// (host, Docker, ECS Fargate) and an AWS Lambda worker. The Lambda path is
// selected at runtime when AWS_LAMBDA_FUNCTION_NAME is set, which the
// Lambda runtime always provides.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/alexandreroman/aws-image-processing-demo/internal/activities"
	"github.com/alexandreroman/aws-image-processing-demo/internal/anthropicclient"
	"github.com/alexandreroman/aws-image-processing-demo/internal/awsclient"
	"github.com/alexandreroman/aws-image-processing-demo/internal/temporalclient"
	"github.com/alexandreroman/aws-image-processing-demo/internal/workflows"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.temporal.io/sdk/contrib/aws/lambdaworker"
	"go.temporal.io/sdk/contrib/sysinfo"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

const (
	defaultTaskQueue      = "image-processing"
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

	taskQueue := temporalclient.EnvOr("TEMPORAL_TASK_QUEUE", defaultTaskQueue)

	// Cap concurrent activities so a large burst cannot exhaust the worker's memory
	// (each Resize holds a decoded RGBA buffer). 4 is the aligned default across all
	// runtimes (host/compose/ECS/Lambda) for predictable burst behavior.
	maxConcurrent := envIntOr("WORKER_MAX_CONCURRENT_ACTIVITIES", 4)
	w := worker.New(tc, taskQueue, worker.Options{
		MaxConcurrentActivityExecutionSize: maxConcurrent,
		// Report real CPU/RAM in worker heartbeats so the Temporal Cloud
		// "Worker Hosts" view can distinguish a busy worker from a downed
		// one. Long-running workers only — the Lambda path doesn't apply.
		SysInfoProvider: sysinfo.SysInfoProvider(),
	})
	registerAll(w, acts)
	if err := w.Start(); err != nil {
		return err
	}

	logger.Info("worker starting",
		"taskQueue", taskQueue,
		"namespace", namespace,
		"bucket", acts.ImagesBucket,
		"table", acts.ImagesTable,
		"maxConcurrentActivities", maxConcurrent,
	)

	// The worker speaks to Temporal over gRPC; this HTTP listener exists
	// purely as a liveness probe for the container orchestrator (compose
	// healthcheck, ECS container healthCheck).
	healthAddr := temporalclient.EnvOr("HEALTH_ADDR", ":8001")
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

	// worker.InterruptCh closes on SIGINT/SIGTERM.
	<-worker.InterruptCh()
	logger.Info("worker stopping", "taskQueue", taskQueue)
	w.Stop()
	return nil
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
	defer tc.Close()

	taskQueue := temporalclient.EnvOr("TEMPORAL_TASK_QUEUE", defaultTaskQueue)
	deploymentName := temporalclient.EnvOr("WORKER_DEPLOYMENT_NAME", defaultDeploymentName)

	acts, err := activities.New(s3c, ddb, anth, tc)
	if err != nil {
		logger.Error("worker: build activities", "err", err)
		os.Exit(1)
	}

	logger.Info("lambda worker starting",
		"taskQueue", taskQueue,
		"namespace", namespace,
		"bucket", acts.ImagesBucket,
		"table", acts.ImagesTable,
		"deploymentName", deploymentName,
		"buildID", buildID,
	)

	version := worker.WorkerDeploymentVersion{
		DeploymentName: deploymentName,
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
		ctx.WorkerOptions.MaxConcurrentActivityExecutionSize = envIntOr("WORKER_MAX_CONCURRENT_ACTIVITIES", 4)
		// Report real CPU/RAM in heartbeats. On Lambda this gives a snapshot
		// of each invocation's container footprint in Temporal Cloud's Worker
		// Hosts view — parity with the long-running path.
		ctx.WorkerOptions.SysInfoProvider = sysinfo.SysInfoProvider()

		registerAll(ctx, acts)

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
