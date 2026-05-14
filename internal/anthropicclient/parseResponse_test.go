package anthropicclient

import (
	"testing"
)

func TestParseResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		raw         string
		wantDesc    string
		wantLabels  []string
		wantErr     bool
		wantErrText string // substring expected in the error message
	}{
		{
			name:       "clean JSON",
			raw:        `{"description":"a dog catching a frisbee","labels":["dog","beach","pet"]}`,
			wantDesc:   "a dog catching a frisbee",
			wantLabels: []string{"dog", "beach", "pet"},
		},
		{
			name: "JSON wrapped in markdown fence",
			raw: "Here is the result:\n```json\n" +
				`{"description":"sunset over mountains","labels":["sunset","mountain","sky"]}` +
				"\n```",
			wantDesc:   "sunset over mountains",
			wantLabels: []string{"sunset", "mountain", "sky"},
		},
		{
			name:       "prose fallback",
			raw:        "Description: a small kitten on a couch\nLabels: kitten, couch, indoor",
			wantDesc:   "a small kitten on a couch",
			wantLabels: []string{"kitten", "couch", "indoor"},
		},
		{
			name:        "empty string",
			raw:         "",
			wantErr:     true,
			wantErrText: "could not parse",
		},
		{
			name:        "malformed JSON without recoverable prose",
			raw:         `{"description": "broken`,
			wantErr:     true,
			wantErrText: "could not parse",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			desc, labels, err := parseResponse(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got desc=%q labels=%v", desc, labels)
				}
				if tc.wantErrText != "" && !contains(err.Error(), tc.wantErrText) {
					t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErrText)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if desc != tc.wantDesc {
				t.Errorf("desc: got %q, want %q", desc, tc.wantDesc)
			}
			if !equalStringSlice(labels, tc.wantLabels) {
				t.Errorf("labels: got %v, want %v", labels, tc.wantLabels)
			}
		})
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
