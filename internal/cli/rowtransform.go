package cli

import (
	"strings"
	"text/template"
)

// RowTransformData is the template data available to a Data-tab "apply
// custom Go template" transform: the current cell's value as `{{.Value}}`.
type RowTransformData struct {
	Value any
}

// RenderRowTransformTemplate runs tmplText through text/template with the
// same generator/formula FuncMap the CLI's SQL templating uses (buildTemplateFuncMap),
// so `{{uuid}}`, `{{upper .Value}}`, etc. all work here exactly as they do in
// `squad cli`. Value is exposed as the template's `.Value` field.
func RenderRowTransformTemplate(tmplText string, value any) (string, error) {
	tmpl, err := template.New("rowtransform").Funcs(buildTemplateFuncMap()).Parse(tmplText)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if err := tmpl.Execute(&b, RowTransformData{Value: value}); err != nil {
		return "", err
	}
	return b.String(), nil
}
