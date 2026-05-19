// Command backend serves the demo's REST API.
//
// Mode selection follows the project convention recorded in project
// memory: if AWS_ENDPOINT_URL is set (LocalStack), boot as a plain HTTP
// server on :8000; otherwise boot as a Lambda handler. A single env var
// drives the choice — no RUN_MODE knob.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexandreroman/aws-image-processing-demo/internal/api"
	"github.com/alexandreroman/aws-image-processing-demo/internal/awsclient"
	"github.com/alexandreroman/aws-image-processing-demo/internal/temporalclient"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
	"go.temporal.io/sdk/client"
)

const (
	httpAddr = ":8000"

	// defaultTaskQueue is the single queue used when no per-runtime queue is
	// configured. This is the local-dev / compose default; the deployed
	// backend Lambda gets WORKER_TASK_QUEUE_ECS and WORKER_TASK_QUEUE_LAMBDA
	// from Tofu and bypasses this fallback entirely.
	defaultTaskQueue = "image-processing"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	handler, tc, err := build(ctx, logger)
	if err != nil {
		logger.Error("backend boot failed", "err", err)
		os.Exit(1)
	}

	if os.Getenv("AWS_ENDPOINT_URL") != "" {
		runHTTP(ctx, logger, handler, tc)
		return
	}
	logger.Info("starting Lambda handler")
	lambda.Start(httpadapter.NewV2(handler).ProxyWithContext)
}

func build(ctx context.Context, logger *slog.Logger) (http.Handler, client.Client, error) {
	awsCfg, err := awsclient.Load(ctx)
	if err != nil {
		return nil, nil, err
	}
	s3c := awsclient.NewS3(awsCfg)
	presigner := s3.NewPresignClient(s3c)
	ddb := awsclient.NewDynamoDB(awsCfg)

	tc, namespace, err := temporalclient.Dial(logger)
	if err != nil {
		return nil, nil, err
	}

	bucket := os.Getenv("IMAGES_BUCKET")
	table := os.Getenv("IMAGES_TABLE")
	if bucket == "" || table == "" {
		return nil, nil, errors.New("backend: IMAGES_BUCKET and IMAGES_TABLE are required")
	}

	runtimes := buildRuntimes()
	defaultQueue := temporalclient.EnvOr("TEMPORAL_TASK_QUEUE", defaultTaskQueue)
	h := api.New(api.Dependencies{
		Temporal:         tc,
		Presigner:        presigner,
		Dynamo:           ddb,
		ImagesBucket:     bucket,
		ImagesTable:      table,
		Runtimes:         runtimes,
		DefaultTaskQueue: defaultQueue,
		Namespace:        namespace,
		Logger:           logger,
	})

	logger.Info("backend ready",
		"bucket", bucket,
		"table", table,
		"defaultTaskQueue", defaultQueue,
		"runtimes", runtimes,
	)
	return h, tc, nil
}

// buildRuntimes resolves the worker runtimes the backend will route to.
// Presence (not value) of WORKER_TASK_QUEUE_ECS / WORKER_TASK_QUEUE_LAMBDA
// is the signal: Tofu sets these on the deployed backend Lambda, so the
// runtime selector only lights up in real deployments. In local dev /
// compose neither var is set and the handler falls back to
// DefaultTaskQueue.
func buildRuntimes() []api.Runtime {
	out := make([]api.Runtime, 0, 2)
	if q := os.Getenv("WORKER_TASK_QUEUE_ECS"); q != "" {
		out = append(out, api.Runtime{Name: "ecs", TaskQueue: q})
	}
	if q := os.Getenv("WORKER_TASK_QUEUE_LAMBDA"); q != "" {
		out = append(out, api.Runtime{Name: "lambda", TaskQueue: q})
	}
	return out
}

func runHTTP(ctx context.Context, logger *slog.Logger, h http.Handler, tc client.Client) {
	defer tc.Close()
	srv := &http.Server{
		Addr:              httpAddr,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	logger.Info("HTTP server listening", "addr", httpAddr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("HTTP server stopped with error", "err", err)
		os.Exit(1)
	}
}
