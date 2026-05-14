// Command worker is the Temporal worker. It long-polls the task queue and
// executes the ProcessImage workflow plus all its activities.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/alexandreroman/aws-image-processing-demo/internal/activities"
	"github.com/alexandreroman/aws-image-processing-demo/internal/anthropicclient"
	"github.com/alexandreroman/aws-image-processing-demo/internal/awsclient"
	"github.com/alexandreroman/aws-image-processing-demo/internal/temporalclient"
	"github.com/alexandreroman/aws-image-processing-demo/internal/workflows"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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
	presigner := s3.NewPresignClient(s3c)

	anth, err := anthropicclient.New()
	if err != nil {
		return err
	}

	acts, err := activities.New(s3c, presigner, ddb, anth, activities.Config{})
	if err != nil {
		return err
	}

	tc, namespace, err := temporalclient.Dial(logger)
	if err != nil {
		return err
	}
	defer tc.Close()

	taskQueue := envOr("TEMPORAL_TASK_QUEUE", defaultTaskQueue)
	w := worker.New(tc, taskQueue, worker.Options{})
	w.RegisterWorkflow(workflows.ProcessImage)
	w.RegisterActivity(acts)

	logger.Info("worker starting",
		"taskQueue", taskQueue,
		"namespace", namespace,
		"bucket", acts.ImagesBucket,
		"table", acts.ImagesTable,
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
