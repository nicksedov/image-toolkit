package imaging

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
	"unicode"
)

//go:embed prompts/*.txt
var promptsFS embed.FS

// promptTemplates holds parsed templates for prompts with placeholders
var promptTemplates = make(map[string]*template.Template)

func init() {
	// Parse all prompt files as templates
	templatedPrompts := []string{
		"prompts/ocr_system.txt",
		"prompts/action_describe.txt",
		"prompts/action_tags.txt",
		"prompts/action_recognize_text.txt",
		"prompts/action_ask_question.txt",
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

// renderPrompt renders a templated prompt with the given data
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

// loadPrompt reads a static prompt file (no template rendering)
func loadPrompt(name string) string {
	content, err := promptsFS.ReadFile(name)
	if err != nil {
		panic(fmt.Sprintf("failed to read embedded prompt %s: %v", name, err))
	}
	return string(content)
}

// languageCodeToName converts a language code (e.g. "en", "ru") to a full language name
func languageCodeToName(code string) string {
	switch code {
	case "ru":
		return "Russian"
	default:
		return "English"
	}
}

// detectQuestionLanguage determines the language of a user question based on character analysis
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
