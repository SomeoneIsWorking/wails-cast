package ai

import "context"

// Provider identifiers for the LLM backend selection.
const (
	ProviderOpenCode    = "opencode"
	ProviderOpenAICompat = "openai-compat"
)

// LLMClient is the minimal interface the translation pipeline requires from any
// LLM backend.  The concrete implementation (Translator) is shared between the
// "opencode" and "openai-compat" providers; callers talk to this interface so
// the backend can be swapped without touching business logic.
type LLMClient interface {
	// StreamCompletion sends a single-turn prompt to the model and returns the
	// full accumulated response text.  Implementations must stream the response
	// and emit "translation:stream" events for each delta so the frontend can
	// display live output.
	StreamCompletion(ctx context.Context, prompt string) (string, error)

	// Close releases any resources held by the client (connections, etc.).
	Close() error
}

// NewLLMClient creates the appropriate LLMClient for the given provider.
//
//   - provider: one of ProviderOpenCode or ProviderOpenAICompat
//   - apiKey:   bearer token for the provider
//   - model:    model name to request
//   - baseURL:  base URL of the OpenAI-compatible endpoint (ignored for
//               ProviderOpenCode, which always uses OpenCodeBaseURL)
func NewLLMClient(provider, apiKey, model, baseURL string) (LLMClient, error) {
	switch provider {
	case ProviderOpenCode, "":
		// opencode always uses the canonical base URL
		return NewTranslator(apiKey, model)
	case ProviderOpenAICompat:
		return NewTranslatorWithBaseURL(apiKey, model, baseURL)
	default:
		// Unknown provider — fall back to opencode so the app stays functional
		return NewTranslator(apiKey, model)
	}
}
