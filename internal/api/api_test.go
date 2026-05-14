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
	})
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
			name:          "size above limit",
			body:          fmt.Sprintf(`{"count":1,"size":%d}`, maxPresignSize+1),
			wantStatus:    http.StatusBadRequest,
			wantErrSubstr: "size must be",
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
