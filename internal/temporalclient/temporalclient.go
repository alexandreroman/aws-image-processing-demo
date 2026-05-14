// Package temporalclient centralizes the Temporal SDK client construction
// shared by the worker and backend binaries. It honors mTLS env vars so
// the same code path works for the local dev server and Temporal Cloud.
package temporalclient

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"

	"go.temporal.io/sdk/client"
	sdklog "go.temporal.io/sdk/log"
)

// Dial connects to Temporal, honoring TEMPORAL_ADDRESS, TEMPORAL_NAMESPACE,
// and optional TEMPORAL_TLS_CERT / TEMPORAL_TLS_KEY for mTLS (Temporal Cloud).
//
// It returns the dialed client and the resolved namespace so callers can log
// and propagate it without re-reading the env.
func Dial(logger *slog.Logger) (client.Client, string, error) {
	address := envOr("TEMPORAL_ADDRESS", client.DefaultHostPort)
	namespace := envOr("TEMPORAL_NAMESPACE", client.DefaultNamespace)

	opts := client.Options{
		HostPort:  address,
		Namespace: namespace,
		Logger:    sdklog.NewStructuredLogger(logger),
	}

	certPath := os.Getenv("TEMPORAL_TLS_CERT")
	keyPath := os.Getenv("TEMPORAL_TLS_KEY")
	if certPath != "" && keyPath != "" {
		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			return nil, "", fmt.Errorf("temporalclient: load mTLS keypair (%s, %s): %w", certPath, keyPath, err)
		}
		opts.ConnectionOptions.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	}

	tc, err := client.Dial(opts)
	if err != nil {
		return nil, "", err
	}
	return tc, namespace, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
