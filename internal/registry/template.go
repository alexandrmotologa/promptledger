package registry

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"text/template"
)

var (
	mustacheVarRegex = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_\.]+)\s*\}\}`)
)

// NormalizeTemplate converts {{Variable}} into {{.Variable}} for standard Go text/template syntax.
func NormalizeTemplate(raw string) string {
	return mustacheVarRegex.ReplaceAllStringFunc(raw, func(match string) string {
		sub := mustacheVarRegex.FindStringSubmatch(match)
		if len(sub) > 1 {
			varName := strings.TrimSpace(sub[1])
			if !strings.HasPrefix(varName, ".") {
				varName = "." + varName
			}
			return "{{" + varName + "}}"
		}
		return match
	})
}

// ExtractTemplateVariables finds all placeholder variable names in the prompt template.
func ExtractTemplateVariables(raw string) []string {
	normalized := NormalizeTemplate(raw)
	matches := mustacheVarRegex.FindAllStringSubmatch(normalized, -1)
	seen := make(map[string]bool)
	var vars []string
	for _, m := range matches {
		if len(m) > 1 {
			name := strings.TrimPrefix(strings.TrimSpace(m[1]), ".")
			if !seen[name] {
				seen[name] = true
				vars = append(vars, name)
			}
		}
	}
	return vars
}

// RenderTemplate evaluates the prompt template with provided key-value variables.
func RenderTemplate(templateStr string, variables map[string]interface{}) (string, error) {
	norm := NormalizeTemplate(templateStr)
	tmpl, err := template.New("prompt").Option("missingkey=error").Parse(norm)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, variables); err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}

	return buf.String(), nil
}
