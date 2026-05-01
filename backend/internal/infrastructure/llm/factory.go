package llm

import "fmt"

// NewClient creates an LLM client based on provider type
func NewClient(provider, baseURL, apiKey, model string) (Client, error) {
	switch provider {
	case ProviderOllama:
		return NewOllamaClient(baseURL, model), nil
	case ProviderOpenAI:
		if apiKey == "" {
			return nil, fmt.Errorf("API key is required for OpenAI provider")
		}
		return NewOpenAIClient(baseURL, apiKey, model), nil
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", provider)
	}
}
