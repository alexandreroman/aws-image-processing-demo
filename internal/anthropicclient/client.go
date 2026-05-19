// Package anthropicclient wraps the Anthropic Go SDK with a single
// Describe call tailored to the demo's image-description need.
package anthropicclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// ErrClaudeInvalidInput is returned when the model refuses or cannot parse
// the input image. The workflow uses this sentinel to mark the failure
// non-retryable (no point retrying a malformed JPEG).
var ErrClaudeInvalidInput = errors.New("claude: invalid input")

// Model is the Claude Haiku 4.5 vision-capable model used for image
// descriptions. Pinned to a dated version for deterministic behavior.
const Model = "claude-haiku-4-5-20251001"

// Client wraps the Anthropic SDK with the demo-specific prompt.
type Client struct {
	api anthropic.Client
}

// New builds a client. The API key is read from ANTHROPIC_API_KEY.
func New() (*Client, error) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return nil, errors.New("anthropicclient: ANTHROPIC_API_KEY is not set")
	}
	c := anthropic.NewClient(option.WithAPIKey(key))
	return &Client{api: c}, nil
}

// Describe asks Claude to caption the given image and return 3-5 single-word
// lowercase labels. mimeType must be a supported image type (image/jpeg,
// image/png, image/gif, image/webp).
func (c *Client) Describe(ctx context.Context, imageBytes []byte, mimeType string) (string, []string, error) {
	if len(imageBytes) == 0 {
		return "", nil, fmt.Errorf("%w: empty image", ErrClaudeInvalidInput)
	}
	if !isSupportedMime(mimeType) {
		return "", nil, fmt.Errorf("%w: unsupported mime type %q", ErrClaudeInvalidInput, mimeType)
	}

	encoded := base64.StdEncoding.EncodeToString(imageBytes)

	prompt := strings.TrimSpace(`
Describe this image in one short sentence (max 15 words) and provide 3 to 5
single-word, lowercase tags. Respond with strict JSON only, matching:
{"description": "...", "labels": ["...", "..."]}
No prose, no markdown fences.
`)

	msg, err := c.api.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     Model,
		MaxTokens: 256,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(
				anthropic.NewImageBlockBase64(mimeType, encoded),
				anthropic.NewTextBlock(prompt),
			),
		},
	})
	if err != nil {
		return "", nil, fmt.Errorf("anthropicclient: messages.new: %w", err)
	}

	raw := firstText(msg)
	if raw == "" {
		return "", nil, fmt.Errorf("%w: empty model response", ErrClaudeInvalidInput)
	}

	desc, labels, err := parseResponse(raw)
	if err != nil {
		return "", nil, fmt.Errorf("%w: %v", ErrClaudeInvalidInput, err)
	}
	return desc, labels, nil
}

func firstText(msg *anthropic.Message) string {
	for _, block := range msg.Content {
		if block.Type == "text" {
			return block.Text
		}
	}
	return ""
}

// parseResponse extracts the description and labels from the model's
// reply. We accept any reply that contains a strict-JSON {...} object
// matching the schema; markdown fences or leading/trailing prose are
// tolerated by isolating the first '{' through the last '}'.
func parseResponse(raw string) (string, []string, error) {
	type payload struct {
		Description string   `json:"description"`
		Labels      []string `json:"labels"`
	}

	start, end := strings.Index(raw, "{"), strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return "", nil, fmt.Errorf("could not parse: %s", truncate(raw, 200))
	}
	var p payload
	if err := json.Unmarshal([]byte(raw[start:end+1]), &p); err != nil || p.Description == "" {
		return "", nil, fmt.Errorf("could not parse: %s", truncate(raw, 200))
	}
	return p.Description, normalizeLabels(p.Labels), nil
}

func normalizeLabels(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.ToLower(strings.TrimSpace(s))
		s = strings.Trim(s, `"' #`)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func isSupportedMime(m string) bool {
	switch m {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return true
	}
	return false
}
