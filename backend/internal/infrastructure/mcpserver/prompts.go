package mcpserver

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
	"unicode"
)

//go:embed prompts/*.txt
var promptsFS embed.FS

// promptTemplates holds parsed templates for MCP tool prompts.
var promptTemplates = make(map[string]*template.Template)

func init() {
	templatedPrompts := []string{
		"prompts/action_describe.txt",
		"prompts/action_tags.txt",
		"prompts/action_recognize_text.txt",
		"prompts/action_ask_question.txt",
		"prompts/action_default.txt",
	}

	for _, name := range templatedPrompts {
		content, err := promptsFS.ReadFile(name)
		if err != nil {
			panic(fmt.Sprintf("failed to read embedded prompt %s: %v", name, err))
		}
		tmpl, err := template.New(name).Parse(string(content))
		if err != nil {
			panic(fmt.Sprintf("failed to parse prompt template %s: %v", name, err))
		}
		promptTemplates[name] = tmpl
	}
}

// renderPrompt renders a templated prompt with the given data.
func renderPrompt(name string, data any) string {
	tmpl, ok := promptTemplates[name]
	if !ok {
		panic(fmt.Sprintf("prompt template not found: %s", name))
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		panic(fmt.Sprintf("failed to execute prompt template %s: %v", name, err))
	}
	return buf.String()
}

// loadPrompt reads a static prompt file (no template rendering).
func loadPrompt(name string) string {
	content, err := promptsFS.ReadFile(name)
	if err != nil {
		panic(fmt.Sprintf("failed to read embedded prompt %s: %v", name, err))
	}
	return string(content)
}

// languageCodeToName converts a language code (e.g. "en", "ru") to a full language name.
func languageCodeToName(code string) string {
	switch code {
	case "ru":
		return "Russian"
	default:
		return "English"
	}
}

// detectQuestionLanguage determines the language of a user question based on character analysis.
func detectQuestionLanguage(text string) string {
	cyrillicCount := 0
	latinCount := 0

	for _, ch := range text {
		if unicode.Is(unicode.Cyrillic, ch) {
			cyrillicCount++
		} else if unicode.Is(unicode.Latin, ch) {
			latinCount++
		}
	}

	if cyrillicCount > latinCount {
		return "Russian"
	}
	return "English"
}

// actionPromptData holds template data for action prompts.
type actionPromptData struct {
	ResponseLanguage string
}

// recognizeTextPromptData holds template data for OCR prompts.
type recognizeTextPromptData struct {
	NoTextMessage string
}

// askQuestionPromptData holds template data for question prompts.
type askQuestionPromptData struct {
	Question         string
	QuestionLanguage string
}

// buildActionPrompt creates the system prompt for an AI action.
func buildActionPrompt(action, question, language string) string {
	responseLang := languageCodeToName(language)

	switch action {
	case "describe":
		return renderPrompt("prompts/action_describe.txt", actionPromptData{ResponseLanguage: responseLang})
	case "tags":
		return loadPrompt("prompts/action_tags.txt")
	case "recognizeText":
		noTextMsg := "No text detected."
		if language == "ru" {
			noTextMsg = "Текст не обнаружен."
		}
		return renderPrompt("prompts/action_recognize_text.txt", recognizeTextPromptData{NoTextMessage: noTextMsg})
	case "askQuestion":
		questionLang := detectQuestionLanguage(question)
		return renderPrompt("prompts/action_ask_question.txt", askQuestionPromptData{
			Question:         question,
			QuestionLanguage: questionLang,
		})
	default:
		return loadPrompt("prompts/action_default.txt")
	}
}

// buildActionUserMessage returns the user message for the LLM based on action type.
func buildActionUserMessage(action string) string {
	switch action {
	case "describe":
		return "Describe this image in detail."
	case "tags":
		return "Generate tags for this image."
	case "recognizeText":
		return "Recognize and extract all text from this image."
	case "askQuestion":
		return "Answer the question about this image."
	default:
		return "Analyze this image."
	}
}
