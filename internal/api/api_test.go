package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestHandler builds a Handler with only the fields handlePresign needs
// for its validation paths. Presigner is left nil because every test case
// here errors out before reaching it.
func newTestHandler() *Handler {
	return New(Dependencies{
		ImagesBucket: "test-bucket",
		Runtimes:     []Runtime{{Name: "ecs", TaskQueue: "image-processing-ecs"}},
	})
}

// newTestHandlerWithoutRuntimes mirrors newTestHandler but leaves Runtimes
// empty so the misconfiguration path can be exercised.
func newTestHandlerWithoutRuntimes() *Handler {
	return New(Dependencies{ImagesBucket: "test-bucket"})
}

func TestHandlePresign_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		body           string
		contentTypeRaw bool // when true, send body verbatim (used for empty body)
		wantStatus     int
		wantErrSubstr  string
	}{
		{
			name:          "negative count",
			body:          `{"count":-1}`,
			wantStatus:    http.StatusBadRequest,
			wantErrSubstr: "count must be between",
		},
		{
			name:          "zero count",
			body:          `{"count":0}`,
			wantStatus:    http.StatusBadRequest,
			wantErrSubstr: "count must be between",
		},
		{
			name:          "count above limit",
			body:          `{"count":51}`,
			wantStatus:    http.StatusBadRequest,
			wantErrSubstr: "count must be between",
		},
		{
			name:          "empty body",
			body:          "",
			wantStatus:    http.StatusBadRequest,
			wantErrSubstr: "invalid body",
		},
		{
			name:          "malformed json",
			body:          `{"count":`,
			wantStatus:    http.StatusBadRequest,
			wantErrSubstr: "invalid body",
		},
	}

	h := newTestHandler()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, "/api/uploads/presign",
				bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status: got %d, want %d (body=%s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantErrSubstr != "" {
				var resp map[string]string
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("decode response: %v (body=%s)", err, rec.Body.String())
				}
				if !strings.Contains(resp["error"], tc.wantErrSubstr) {
					t.Fatalf("error %q does not contain %q", resp["error"], tc.wantErrSubstr)
				}
			}
		})
	}
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
			name:          "wrong bucket",
			body:          `{"images":[{"bucket":"someone-elses-bucket","key":"uploads/x.jpg"}]}`,
			wantErrSubstr: "must match the configured images bucket",
		},
		{
			name:          "forbidden key prefix pipelines",
			body:          `{"images":[{"bucket":"test-bucket","key":"pipelines/foo.jpg"}]}`,
			wantErrSubstr: "must start with uploads/ or samples/",
		},
		{
			name:          "forbidden key prefix bare",
			body:          `{"images":[{"bucket":"test-bucket","key":"evil"}]}`,
			wantErrSubstr: "must start with uploads/ or samples/",
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
	refs := make([]string, maxPresignCnt+1)
	for i := range refs {
		refs[i] = `{"bucket":"test-bucket","key":"uploads/x.jpg"}`
	}
	body := `{"images":[` + strings.Join(refs, ",") + `]}`

	h := newTestHandler()
	status, gotErr := postJSON(t, h, "/api/workflows/start", body)
	if status != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d (err=%q)", status, http.StatusBadRequest, gotErr)
	}
	wantSubstr := fmt.Sprintf("at most %d", maxPresignCnt)
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
	body := `{"images":[{"bucket":"test-bucket","key":"uploads/x.jpg"}],"runtime":"firecracker"}`
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

func TestHandlePresign_CountErrorMentionsActualValue(t *testing.T) {
	t.Parallel()

	h := newTestHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/uploads/presign",
		bytes.NewBufferString(`{"count":999}`))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !strings.Contains(resp["error"], "999") {
		t.Fatalf("expected error to mention the offending count, got %q", resp["error"])
	}
}
