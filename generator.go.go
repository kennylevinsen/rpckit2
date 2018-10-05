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

//go:generate go-bindata --pkg rpckit2 -o templates.go templates/...

var template_deps = []string{
	"rpckit_boilerplate.go.tmpl",
	"rpckit_property_to_wiretype.go.tmpl",
	"rpckit_serialization.go.tmpl",
	"rpckit_client_definition.go.tmpl",
	"rpckit_client_method.go.tmpl",
	"rpckit_server_method.go.tmpl",
	"rpckit_rpccall.go.tmpl",
	"rpckit.go.tmpl",
}

type GoGenerator struct {
	PackageName string
	Protocols   []*Protocol
}

type TemplateProtocol struct {
	Name    string
	ID      uint64
	Methods []Method
}

type TemplateContext struct {
	PackageName string
	Protocols   []TemplateProtocol
}

func (g GoGenerator) Generate(p string) error {
	funcs := template.FuncMap{
		"error": func(s string) error {
			return errors.New(s)
		},
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
		b, err := Asset(path.Join("templates/go-pb", name))
		if err != nil {
			return fmt.Errorf("template reading failed: %+v\n", err)
		}
		var t *template.Template
		t = tmpl.New(name)
		if _, err := t.Parse(string(b)); err != nil {
			return fmt.Errorf("template parsing failed: %+v\n", err)
		}
	}

	ctx := TemplateContext{
		PackageName: g.PackageName,
	}

	for _, v := range g.Protocols {
		ctx.Protocols = append(ctx.Protocols, TemplateProtocol{
			Name:    v.name,
			ID:      v.id,
			Methods: v.methods,
		})
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "rpckit", ctx); err != nil {
		return fmt.Errorf("template execution failed: %+v\n", err)
	}

	return ioutil.WriteFile(p, buf.Bytes(), 0644)
}
