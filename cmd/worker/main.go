// Command worker is the Temporal worker. It long-polls the task queue and
// executes the ProcessImage workflow plus all its activities.
package main

import (
	"context"
	"log/slog"
	"os"
	"strconv"

	"github.com/alexandreroman/aws-image-processing-demo/internal/activities"
	"github.com/alexandreroman/aws-image-processing-demo/internal/anthropicclient"
	"github.com/alexandreroman/aws-image-processing-demo/internal/awsclient"
	"github.com/alexandreroman/aws-image-processing-demo/internal/temporalclient"
	"github.com/alexandreroman/aws-image-processing-demo/internal/workflows"
	"go.temporal.io/sdk/worker"
)

const defaultTaskQueue = "image-processing"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("worker exited with error", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	// AWS config loading is fast and only needs a Background context.
	// Cancellation flows through worker.InterruptCh() below.
	awsCfg, err := awsclient.Load(context.Background())
	if err != nil {
		return err
	}
	s3c := awsclient.NewS3(awsCfg)
	ddb := awsclient.NewDynamoDB(awsCfg)

	anth, err := anthropicclient.New()
	if err != nil {
		return err
	}

	tc, namespace, err := temporalclient.Dial(logger)
	if err != nil {
		return err
	}
	defer tc.Close()

	taskQueue := envOr("TEMPORAL_TASK_QUEUE", defaultTaskQueue)
	acts, err := activities.New(s3c, ddb, anth, tc, activities.Config{TaskQueue: taskQueue})
	if err != nil {
		return err
	}

	// Cap concurrent activities so a large burst cannot exhaust the
	// worker's memory (each Resize holds a decoded RGBA buffer). 8 is
	// safe for the 256 MiB compose container; prod (1+ GiB Fargate)
	// overrides via WORKER_MAX_CONCURRENT_ACTIVITIES.
	maxConcurrent := envIntOr("WORKER_MAX_CONCURRENT_ACTIVITIES", 8)
	w := worker.New(tc, taskQueue, worker.Options{
		MaxConcurrentActivityExecutionSize: maxConcurrent,
	})
	w.RegisterWorkflow(workflows.ProcessImage)
	w.RegisterWorkflow(workflows.LaunchPipelines)
	w.RegisterActivity(acts)

	logger.Info("worker starting",
		"taskQueue", taskQueue,
		"namespace", namespace,
		"bucket", acts.ImagesBucket,
		"table", acts.ImagesTable,
		"maxConcurrentActivities", maxConcurrent,
	)

	// worker.InterruptCh closes on SIGINT/SIGTERM; no goroutine to leak.
	return w.Run(worker.InterruptCh())
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
