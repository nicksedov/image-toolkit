package llm

// Client interface for VL LLM communication
type Client interface {
	// Recognize performs OCR recognition on an image with given system prompt
	// Returns markdown content and error
	Recognize(imagePath string, systemPrompt string) (string, error)
}

// Provider type enumeration
const (
	ProviderOllama = "ollama"
	ProviderOpenAI = "openai"
)
