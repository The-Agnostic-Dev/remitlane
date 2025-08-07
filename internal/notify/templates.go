package notify

import (
	"bytes"
	"text/template"
)

var tmpl = template.Must(template.New("mail").Parse(`
{{define "recon"}}Hello Finance,

The reconciliation report for {{.Date}} is ready.

Payouts: {{.Count}}
Completed: {{.Completed}}
Failed: {{.Failed}}
USD value: {{.USD}}
{{range .Currencies}}
{{.Currency}}: {{.Count}} payouts, {{.Amount}} paid out, fees {{.Fees}}{{end}}

Download the CSV from the console: {{.Link}}

Remitlane
{{end}}
`))

// Render renders a named template.
func Render(name string, data any) (string, error) {
	var b bytes.Buffer
	if err := tmpl.ExecuteTemplate(&b, name, data); err != nil {
		return "", err
	}
	return b.String(), nil
}
