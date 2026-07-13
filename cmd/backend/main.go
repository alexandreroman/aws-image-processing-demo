// Command backend serves the demo's REST API.
//
// If AWS_ENDPOINT_URL is set (Moto), run as HTTP on the port from PORT
// (default :8000); otherwise start the Lambda handler.
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
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
	"go.temporal.io/sdk/client"
)

// httpAddr returns the HTTP listen address, honoring the PORT env var so
// host-side dev (e.g. `make dev` under Casper/cmux) can bind a remapped port.
// Defaults to :8000 when PORT is unset.
func httpAddr() string {
	if port := os.Getenv("PORT"); port != "" {
		return ":" + port
	}
	return ":8000"
}

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
	h := api.New(api.Dependencies{
		Temporal:     tc,
		Dynamo:       ddb,
		ImagesBucket: bucket,
		ImagesTable:  table,
		Runtimes:     runtimes,
		Namespace:    namespace,
		Logger:       logger,
	})

	logger.Info("backend ready",
		"bucket", bucket,
		"table", table,
		"runtimes", runtimes,
	)
	return h, tc, nil
}

// buildRuntimes resolves the worker runtimes the backend will route to.
// Presence (not value) of WORKER_TASK_QUEUE_ECS / WORKER_TASK_QUEUE_LAMBDA
// is the signal: Tofu sets these on the deployed backend Lambda, so the
// runtime selector only lights up in real deployments. In local dev /
// compose neither var is set and the API falls back to its built-in
// defaultTaskQueue.
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
	addr := httpAddr()
	srv := &http.Server{
		Addr:              addr,
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

	logger.Info("HTTP server listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("HTTP server stopped with error", "err", err)
		os.Exit(1)
	}
}
