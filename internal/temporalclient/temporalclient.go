// Package temporalclient centralizes the Temporal SDK client construction
// shared by the worker and backend binaries. It honors mTLS env vars so
// the same code path works for the local dev server and Temporal Cloud.
package temporalclient

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"go.temporal.io/sdk/client"
	sdklog "go.temporal.io/sdk/log"
)

// pemHeader marks the start of any PEM block. We use it to distinguish
// inline PEM content (injected via Secrets Manager in ECS/Lambda) from
// filesystem paths (used by local dev or future container mounts).
const pemHeader = "-----BEGIN"

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

	certVar := os.Getenv("TEMPORAL_TLS_CERT")
	keyVar := os.Getenv("TEMPORAL_TLS_KEY")
	if certVar != "" && keyVar != "" {
		cert, err := loadKeyPair(certVar, keyVar)
		if err != nil {
			return nil, "", err
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

// loadKeyPair builds a TLS certificate from TEMPORAL_TLS_CERT / TEMPORAL_TLS_KEY
// values that may be either inline PEM content or filesystem paths. PEM content
// is detected by the presence of a "-----BEGIN" header — both vars must use the
// same form. Mixing inline PEM and a path is rejected so misconfigurations fail
// loudly instead of silently falling back.
func loadKeyPair(certVar, keyVar string) (tls.Certificate, error) {
	certInline := strings.Contains(certVar, pemHeader)
	keyInline := strings.Contains(keyVar, pemHeader)

	switch {
	case certInline && keyInline:
		cert, err := tls.X509KeyPair([]byte(certVar), []byte(keyVar))
		if err != nil {
			return tls.Certificate{}, fmt.Errorf("temporalclient: parse inline mTLS keypair from TEMPORAL_TLS_CERT/TEMPORAL_TLS_KEY: %w", err)
		}
		return cert, nil
	case !certInline && !keyInline:
		cert, err := tls.LoadX509KeyPair(certVar, keyVar)
		if err != nil {
			return tls.Certificate{}, fmt.Errorf("temporalclient: load mTLS keypair from files (%s, %s): %w", certVar, keyVar, err)
		}
		return cert, nil
	default:
		return tls.Certificate{}, fmt.Errorf("temporalclient: TEMPORAL_TLS_CERT and TEMPORAL_TLS_KEY must use the same form (both inline PEM or both file paths)")
	}
}
