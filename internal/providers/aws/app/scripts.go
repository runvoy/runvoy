package aws

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed templates/*.tmpl
var scriptTemplates embed.FS

var scripts = template.Must(
	template.New("scripts").
		Option("missingkey=error").
		ParseFS(scriptTemplates, "templates/*.tmpl"),
)

func renderScript(templateName string, data any) string {
	var buf bytes.Buffer
	if err := scripts.ExecuteTemplate(&buf, templateName, data); err != nil {
		panic(fmt.Errorf("failed to render script template %q: %w", templateName, err))
	}
	return strings.TrimSpace(buf.String())
}
