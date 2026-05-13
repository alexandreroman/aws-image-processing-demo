// Command worker is the Temporal worker. It long-polls the task queue and
// executes the ProcessImage workflow plus all its activities.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/alexandreroman/aws-image-processing-demo/internal/activities"
	"github.com/alexandreroman/aws-image-processing-demo/internal/anthropicclient"
	"github.com/alexandreroman/aws-image-processing-demo/internal/awsclient"
	"github.com/alexandreroman/aws-image-processing-demo/internal/workflows"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.temporal.io/sdk/client"
	sdklog "go.temporal.io/sdk/log"
	"go.temporal.io/sdk/worker"
)

const defaultTaskQueue = "image-processing"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, logger); err != nil {
		logger.Error("worker exited with error", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger) error {
	awsCfg, err := awsclient.Load(ctx)
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

	tc, err := client.Dial(client.Options{
		HostPort:  envOr("TEMPORAL_ADDRESS", client.DefaultHostPort),
		Namespace: envOr("TEMPORAL_NAMESPACE", client.DefaultNamespace),
		Logger:    sdklog.NewStructuredLogger(logger),
	})
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
		"namespace", envOr("TEMPORAL_NAMESPACE", client.DefaultNamespace),
		"bucket", acts.ImagesBucket,
		"table", acts.ImagesTable,
	)

	// worker.Run blocks until interruptCh receives or the worker fails to
	// start. Pipe SIGINT/SIGTERM through the context channel.
	return w.Run(workerSignal(ctx))
}

func workerSignal(ctx context.Context) <-chan interface{} {
	ch := make(chan interface{}, 1)
	go func() {
		<-ctx.Done()
		ch <- struct{}{}
	}()
	return ch
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
