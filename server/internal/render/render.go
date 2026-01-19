package render

import (
	"bytes"
	_ "embed"
	"html"
	"path/filepath"
	"strings"
)

//go:embed templates/markdown.html
var markdownTemplate string

//go:embed templates/code.html
var codeTemplate string

// CanRender returns true if the content type can be rendered
func CanRender(contentType string) bool {
	ct := strings.ToLower(contentType)

	// Markdown
	if strings.Contains(ct, "markdown") {
		return true
	}

	// Code/text files that benefit from syntax highlighting
	if strings.HasPrefix(ct, "text/") && !strings.Contains(ct, "html") {
		return true
	}

	// Common code types
	codeTypes := []string{
		"application/json",
		"application/javascript",
		"application/typescript",
		"application/x-yaml",
		"application/xml",
	}
	for _, t := range codeTypes {
		if strings.Contains(ct, t) {
			return true
		}
	}

	return false
}

// Render converts content to HTML for browser display
func Render(contentType string, content []byte, filename string, id string) ([]byte, error) {
	ct := strings.ToLower(contentType)

	// Markdown
	if strings.Contains(ct, "markdown") {
		return renderMarkdown(content, filename, id)
	}

	// Everything else as code with syntax highlighting
	return renderCode(content, filename, detectLanguage(contentType, filename), id)
}

func renderMarkdown(content []byte, filename string, id string) ([]byte, error) {
	title := filename
	if title == "" {
		title = "Shared Content"
	}

	// Escape content for embedding in JavaScript template literal
	escaped := escapeForJSTemplateLiteral(string(content))

	result := strings.ReplaceAll(markdownTemplate, "{{TITLE}}", html.EscapeString(title))
	result = strings.ReplaceAll(result, "{{CONTENT}}", escaped)
	result = strings.ReplaceAll(result, "{{ID}}", id)

	return []byte(result), nil
}

// escapeForJSTemplateLiteral escapes content for safe embedding in JS template literals
func escapeForJSTemplateLiteral(s string) string {
	// Escape backslashes first, then backticks, then ${
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "`", "\\`")
	s = strings.ReplaceAll(s, "${", "\\${")
	return s
}

func renderCode(content []byte, filename string, language string, id string) ([]byte, error) {
	title := filename
	if title == "" {
		title = "Shared Content"
	}

	// Escape content for embedding in HTML
	escaped := html.EscapeString(string(content))

	result := strings.ReplaceAll(codeTemplate, "{{TITLE}}", html.EscapeString(title))
	result = strings.ReplaceAll(result, "{{LANGUAGE}}", language)
	result = strings.ReplaceAll(result, "{{CONTENT}}", escaped)
	result = strings.ReplaceAll(result, "{{ID}}", id)

	return []byte(result), nil
}

func detectLanguage(contentType string, filename string) string {
	// Try by content type
	ct := strings.ToLower(contentType)
	switch {
	case strings.Contains(ct, "javascript"):
		return "javascript"
	case strings.Contains(ct, "typescript"):
		return "typescript"
	case strings.Contains(ct, "json"):
		return "json"
	case strings.Contains(ct, "yaml"):
		return "yaml"
	case strings.Contains(ct, "xml"):
		return "xml"
	case strings.Contains(ct, "python"):
		return "python"
	case strings.Contains(ct, "go"):
		return "go"
	case strings.Contains(ct, "rust"):
		return "rust"
	case strings.Contains(ct, "ruby"):
		return "ruby"
	case strings.Contains(ct, "shell"):
		return "bash"
	case strings.Contains(ct, "sql"):
		return "sql"
	case strings.Contains(ct, "dockerfile"):
		return "dockerfile"
	}

	// Try by extension
	if filename != "" {
		ext := strings.ToLower(filepath.Ext(filename))
		switch ext {
		case ".js":
			return "javascript"
		case ".ts":
			return "typescript"
		case ".tsx":
			return "tsx"
		case ".jsx":
			return "jsx"
		case ".json":
			return "json"
		case ".yaml", ".yml":
			return "yaml"
		case ".xml":
			return "xml"
		case ".py":
			return "python"
		case ".go":
			return "go"
		case ".rs":
			return "rust"
		case ".rb":
			return "ruby"
		case ".sh", ".bash":
			return "bash"
		case ".sql":
			return "sql"
		case ".html", ".htm":
			return "html"
		case ".css":
			return "css"
		case ".scss":
			return "scss"
		case ".java":
			return "java"
		case ".c", ".h":
			return "c"
		case ".cpp", ".hpp", ".cc":
			return "cpp"
		case ".cs":
			return "csharp"
		case ".php":
			return "php"
		case ".swift":
			return "swift"
		case ".kt":
			return "kotlin"
		case ".scala":
			return "scala"
		case ".hs":
			return "haskell"
		case ".ex", ".exs":
			return "elixir"
		case ".toml":
			return "toml"
		case ".ini":
			return "ini"
		case ".dockerfile":
			return "dockerfile"
		case ".makefile", ".mk":
			return "makefile"
		}
	}

	// Default - let highlight.js auto-detect
	return ""
}

// Simple check if content looks like it might be binary
func IsBinary(content []byte) bool {
	// Check first 512 bytes for null bytes
	checkLen := 512
	if len(content) < checkLen {
		checkLen = len(content)
	}

	return bytes.Contains(content[:checkLen], []byte{0})
}
