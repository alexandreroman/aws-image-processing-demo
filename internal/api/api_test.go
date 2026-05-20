package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestHandler builds a Handler with only the fields handleStart needs
// for its validation paths.
func newTestHandler() *Handler {
	return New(Dependencies{
		ImagesBucket: "test-bucket",
		Runtimes:     []Runtime{{Name: "ecs", TaskQueue: "image-processing-ecs"}},
	})
}

// newTestHandlerWithoutRuntimes mirrors newTestHandler but leaves Runtimes
// empty so the local-dev fallback path can be exercised.
func newTestHandlerWithoutRuntimes() *Handler {
	return New(Dependencies{ImagesBucket: "test-bucket"})
}

// postJSON drives the handler with a JSON body and decodes the {"error": ...}
// response. The validation paths exercised here reject before any Temporal
// RPC, so the nil Temporal client in newTestHandler is never reached.
func postJSON(t *testing.T, h *Handler, path, body string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var resp map[string]string
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v (body=%s)", err, rec.Body.String())
		}
	}
	return rec.Code, resp["error"]
}

func TestHandleStart_RejectsBadS3Refs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		body          string
		wantErrSubstr string
	}{
		{
			name:          "forbidden key prefix pipelines",
			body:          `{"images":[{"key":"pipelines/foo.jpg"}]}`,
			wantErrSubstr: "must start with samples/",
		},
		{
			name:          "forbidden key prefix uploads",
			body:          `{"images":[{"key":"uploads/foo.jpg"}]}`,
			wantErrSubstr: "must start with samples/",
		},
		{
			name:          "forbidden key prefix bare",
			body:          `{"images":[{"key":"evil"}]}`,
			wantErrSubstr: "must start with samples/",
		},
	}

	h := newTestHandler()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			status, gotErr := postJSON(t, h, "/api/workflows/start", tc.body)
			if status != http.StatusBadRequest {
				t.Fatalf("status: got %d, want %d (err=%q)", status, http.StatusBadRequest, gotErr)
			}
			if !strings.Contains(gotErr, tc.wantErrSubstr) {
				t.Fatalf("error %q does not contain %q", gotErr, tc.wantErrSubstr)
			}
		})
	}
}

func TestHandleStart_RejectsBurstAboveCap(t *testing.T) {
	t.Parallel()

	// Build N otherwise-valid refs so the cap check fires before per-image
	// validation has a chance to reject anything.
	refs := make([]string, maxBurst+1)
	for i := range refs {
		refs[i] = `{"key":"samples/1.jpg"}`
	}
	body := `{"images":[` + strings.Join(refs, ",") + `]}`

	h := newTestHandler()
	status, gotErr := postJSON(t, h, "/api/workflows/start", body)
	if status != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d (err=%q)", status, http.StatusBadRequest, gotErr)
	}
	wantSubstr := fmt.Sprintf("at most %d", maxBurst)
	if !strings.Contains(gotErr, wantSubstr) {
		t.Fatalf("error %q does not contain %q", gotErr, wantSubstr)
	}
}

func TestHandleStart_RejectsEmptyImages(t *testing.T) {
	t.Parallel()

	h := newTestHandler()
	status, gotErr := postJSON(t, h, "/api/workflows/start", `{"images":[],"runtime":"ecs"}`)
	if status != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d (err=%q)", status, http.StatusBadRequest, gotErr)
	}
	if !strings.Contains(gotErr, "images must not be empty") {
		t.Fatalf("error %q does not contain %q", gotErr, "images must not be empty")
	}
}

func TestHandleStart_RejectsUnknownRuntime(t *testing.T) {
	t.Parallel()

	h := newTestHandler()
	body := `{"images":[{"key":"samples/1.jpg"}],"runtime":"firecracker"}`
	status, gotErr := postJSON(t, h, "/api/workflows/start", body)
	if status != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d (err=%q)", status, http.StatusBadRequest, gotErr)
	}
	if !strings.Contains(gotErr, `"firecracker"`) {
		t.Fatalf("error %q does not echo the offending runtime", gotErr)
	}
	if !strings.Contains(gotErr, "ecs") {
		t.Fatalf("error %q does not list allowed runtimes", gotErr)
	}
}

func TestHandleRuntimes(t *testing.T) {
	t.Parallel()

	t.Run("configured", func(t *testing.T) {
		t.Parallel()

		h := newTestHandler()
		req := httptest.NewRequest(http.MethodGet, "/api/runtimes", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
		}
		var got []map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode response: %v (body=%s)", err, rec.Body.String())
		}
		if len(got) != 1 || got[0]["name"] != "ecs" {
			t.Fatalf("unexpected payload: %s", rec.Body.String())
		}
		if _, leaked := got[0]["taskQueue"]; leaked {
			t.Fatalf("taskQueue must not be exposed; got %s", rec.Body.String())
		}
	})

	t.Run("unconfigured returns empty array", func(t *testing.T) {
		t.Parallel()

		h := newTestHandlerWithoutRuntimes()
		req := httptest.NewRequest(http.MethodGet, "/api/runtimes", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
		}
		// Distinguish "[]" (empty array) from "null" — the frontend iterates,
		// so a null body would crash it.
		if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
			t.Fatalf("body: got %q, want %q", got, "[]")
		}
	})
}

// TestHandleStart_LocalDevPath exercises the local-dev path where Runtimes
// is empty and the handler falls back to the built-in defaultTaskQueue
// constant. We only assert the validation rejections that fire BEFORE the
// Temporal call — the happy path would NPE on the nil client.
func TestHandleStart_LocalDevPath(t *testing.T) {
	t.Parallel()

	t.Run("empty images still rejected", func(t *testing.T) {
		t.Parallel()

		h := newTestHandlerWithoutRuntimes()
		status, gotErr := postJSON(t, h, "/api/workflows/start", `{"images":[]}`)
		if status != http.StatusBadRequest {
			t.Fatalf("status: got %d, want %d (err=%q)", status, http.StatusBadRequest, gotErr)
		}
		if !strings.Contains(gotErr, "images must not be empty") {
			t.Fatalf("error %q does not contain %q", gotErr, "images must not be empty")
		}
	})

	t.Run("bad key still rejected", func(t *testing.T) {
		t.Parallel()

		h := newTestHandlerWithoutRuntimes()
		body := `{"images":[{"key":"pipelines/foo.jpg"}]}`
		status, gotErr := postJSON(t, h, "/api/workflows/start", body)
		if status != http.StatusBadRequest {
			t.Fatalf("status: got %d, want %d (err=%q)", status, http.StatusBadRequest, gotErr)
		}
		if !strings.Contains(gotErr, "must start with samples/") {
			t.Fatalf("error %q does not mention the key prefix", gotErr)
		}
	})
}

func TestPipelineTiming_AllCompleted(t *testing.T) {
	created := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	latest := created.Add(3500 * time.Millisecond)
	now := created.Add(time.Hour)

	completedAt, durationMs := pipelineTiming(created, latest, 0, 4, now)
	if completedAt == nil {
		t.Fatal("completedAt: got nil, want non-nil")
	}
	if !completedAt.Equal(latest) {
		t.Fatalf("completedAt: got %s, want %s", completedAt, latest)
	}
	if durationMs == nil || *durationMs != 3500 {
		t.Fatalf("durationMs: got %v, want 3500", durationMs)
	}
}

func TestPipelineTiming_SomeRunning(t *testing.T) {
	created := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	now := created.Add(1200 * time.Millisecond)

	completedAt, durationMs := pipelineTiming(created, time.Time{}, 2, 4, now)
	if completedAt != nil {
		t.Fatalf("completedAt: got %s, want nil", completedAt)
	}
	if durationMs == nil || *durationMs != 1200 {
		t.Fatalf("durationMs: got %v, want 1200", durationMs)
	}
}
