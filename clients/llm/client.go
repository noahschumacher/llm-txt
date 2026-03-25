package llm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/anthropics/anthropic-sdk-go"
	anthropicopt "github.com/anthropics/anthropic-sdk-go/option"
	"github.com/openai/openai-go"
	openaiopt "github.com/openai/openai-go/option"
)

const (
	anthropicProvider = "anthropic"
	openaiProvider    = "openai"
)

// Describer generates a one-sentence description of a web page from its body text.
type Describer interface {
	Describe(ctx context.Context, body string) (string, error)
}

// New returns a Describer for the given provider ("anthropic" or "openai").
func New(provider, apiKey, model string, log *zap.Logger) (Describer, error) {
	switch provider {
	case anthropicProvider:
		return &anthropicClient{
			client: anthropic.NewClient(anthropicopt.WithAPIKey(apiKey)),
			model:  model,
			log:    log,
		}, nil
	case openaiProvider:
		return &openaiClient{
			client: openai.NewClient(openaiopt.WithAPIKey(apiKey)),
			model:  model,
			log:    log,
		}, nil
	default:
		return nil, fmt.Errorf("unknown llm provider %q: must be anthropic or openai", provider)
	}
}

// describePrompt is sent to the LLM for every page.
const describePrompt = `Write a one-sentence description of this web page suitable for an llms.txt index.
Be specific and concise. Reply with only the description — no preamble, no quotes.

<content>
%s
</content>`

// maxBodyChars caps the body sent to the LLM to control token cost.
const maxBodyChars = 3000

// -----------------------------------------------------------------------------
// Anthropic

type anthropicClient struct {
	client anthropic.Client
	model  string
	log    *zap.Logger
}

func (c *anthropicClient) Describe(ctx context.Context, body string) (string, error) {
	start := time.Now()
	c.log.Debug("llm call", zap.String("provider", anthropicProvider), zap.String("model", c.model), zap.Int("body_chars", len(body)))

	msg, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: 256,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(
				anthropic.NewTextBlock(
					fmt.Sprintf(describePrompt, truncate(body, maxBodyChars)),
				),
			),
		},
	})
	if err != nil {
		return "", err
	}
	if len(msg.Content) == 0 {
		return "", fmt.Errorf("empty response from anthropic")
	}
	result := strings.TrimSpace(msg.Content[0].AsText().Text)
	c.log.Debug("llm response", zap.String("provider", anthropicProvider), zap.Duration("duration", time.Since(start)), zap.String("result", result))
	return result, nil
}

// -----------------------------------------------------------------------------
// OpenAI

type openaiClient struct {
	client openai.Client
	model  string
	log    *zap.Logger
}

func (c *openaiClient) Describe(ctx context.Context, body string) (string, error) {
	start := time.Now()
	c.log.Debug("llm call", zap.String("provider", openaiProvider), zap.String("model", c.model), zap.Int("body_chars", len(body)))

	resp, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: c.model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(fmt.Sprintf(describePrompt, truncate(body, maxBodyChars))),
		},
	})
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("empty response from openai")
	}
	result := strings.TrimSpace(resp.Choices[0].Message.Content)
	c.log.Debug("llm response", zap.String("provider", openaiProvider), zap.Duration("duration", time.Since(start)), zap.String("result", result))
	return result, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
