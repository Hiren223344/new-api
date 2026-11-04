package emailtpl

import (
	"bytes"
	"embed"
	"html/template"
)

//go:embed *.html
var files embed.FS

var tpl = template.Must(template.ParseFS(files, "*.html"))

func Render(name string, data any) (string, error) {
	var b bytes.Buffer
	if err := tpl.ExecuteTemplate(&b, name, data); err != nil {
		return "", err
	}
	return b.String(), nil
}
