package rpckit2

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"path"
	"strings"
	"text/template"
)

var template_deps = []string{
	"rpckit_header.go.tmpl",
	"rpckit_boilerplate.go.tmpl",
	"rpckit_serialization.go.tmpl",
	"rpckit_client_method.go.tmpl",
	"rpckit_server_method.go.tmpl",
	"rpckit_rpccall.go.tmpl",
	"rpckit.go.tmpl",
}

type GoGenerator struct {
	PackageName string
	Protocol    *Protocol
}

type TemplateContext struct {
	PackageName string
	Name        string
	Methods     []Method
}

func (g GoGenerator) Generate(p string) error {
	funcs := template.FuncMap{
		"capitalize": strings.Title,
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, errors.New("invalid dict call")
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, errors.New("dict keys must be strings")
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},
	}

	tmpl := template.New("").Funcs(funcs)
	for _, name := range template_deps {
		b, err := ioutil.ReadFile(path.Join("../../templates/go", name))
		if err != nil {
			return fmt.Errorf("template reading failed: %+v\n", err)
		}
		var t *template.Template
		t = tmpl.New(name)
		if _, err := t.Parse(string(b)); err != nil {
			return fmt.Errorf("template parsing failed: %+v\n", err)
		}
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "rpckit", TemplateContext{
		PackageName: g.PackageName,
		Name:        g.Protocol.name,
		Methods:     g.Protocol.methods,
	}); err != nil {
		return fmt.Errorf("template execution failed: %+v\n", err)
	}

	return ioutil.WriteFile(p, buf.Bytes(), 0644)
}
