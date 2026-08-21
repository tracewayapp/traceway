// Package emailtemplate renders every outgoing Traceway email through one
// embedded HTML layout so they all share the same look: centered logo,
// headline, white content card with optional badge/details/code block, a
// single action button, a plain-link fallback and a footer.
package emailtemplate

import (
	"bytes"
	_ "embed"
	"html/template"
	"strings"
)

//go:embed base.gohtml
var baseTemplateSource string

var baseTemplate = template.Must(template.New("base").Funcs(template.FuncMap{
	"nl2br": func(s string) template.HTML {
		return template.HTML(strings.ReplaceAll(template.HTMLEscapeString(s), "\n", "<br>"))
	},
}).Parse(baseTemplateSource))

const (
	ColorPrimary  = "#1f883d"
	ColorCritical = "#cf222e"
	ColorWarning  = "#9a6700"
	ColorInfo     = "#0969da"
)

type Detail struct {
	Label string
	Value string
}

type Button struct {
	Label string
	URL   string
}

type Data struct {
	// Preheader is the hidden inbox preview text.
	Preheader string
	LogoURL   string
	Title     string
	// Badge is an optional severity chip above the content (e.g. "CRITICAL").
	Badge      string
	BadgeColor string
	Paragraphs []string
	Details    []Detail
	CodeBlock  string
	Button     *Button
	// ButtonColor defaults to ColorPrimary.
	ButtonColor string
	FooterNote  string
}

func Render(d Data) (string, error) {
	if d.ButtonColor == "" {
		d.ButtonColor = ColorPrimary
	}
	if d.BadgeColor == "" {
		d.BadgeColor = ColorInfo
	}
	if d.Preheader == "" && len(d.Paragraphs) > 0 {
		d.Preheader = d.Paragraphs[0]
	}

	var buf bytes.Buffer
	if err := baseTemplate.Execute(&buf, d); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// LogoURL builds the logo image reference served by the dashboard.
func LogoURL(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/traceway-mark.png"
}
