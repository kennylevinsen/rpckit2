package rpckit2

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"path"
	"strconv"
	"strings"
	"text/template"

	"golang.org/x/tools/imports"
)

//go:generate go-bindata --pkg rpckit2 -o templates.go templates/...

var template_deps = []string{
	"go-pb/boilerplate.go.tmpl",
	"go-pb/property_to_wiretype.go.tmpl",
	"go-pb/serialization.go.tmpl",
	"go-pb/client_definition.go.tmpl",
	"go-pb/client_method.go.tmpl",
	"go-pb/server_method.go.tmpl",
	"go-pb/rpckit.go.tmpl",

	"go-http/boilerplate.go.tmpl",
	"go-http/serialization.go.tmpl",
	"go-http/client_definition.go.tmpl",
	"go-http/client_method.go.tmpl",
	"go-http/server_method.go.tmpl",
	"go-http/rpckit.go.tmpl",

	"go/server_method.go.tmpl",
	"go/rpckit.go.tmpl",
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
		"capitalize":  strings.Title,
		"upper": strings.ToTitle,
		"lower": strings.ToLower,
		"doublequote": strconv.Quote,
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
		b, err := Asset(path.Join("templates", name))
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

	var pbbuf, httpbuf, generalbuf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&pbbuf, "go-pb/rpckit.go.tmpl", ctx); err != nil {
		return fmt.Errorf("pb template execution failed: %+v\n", err)
	}

	if err := tmpl.ExecuteTemplate(&httpbuf, "go-http/rpckit.go.tmpl", ctx); err != nil {
		return fmt.Errorf("http template execution failed: %+v\n", err)
	}

	if err := tmpl.ExecuteTemplate(&generalbuf, "go/rpckit.go.tmpl", ctx); err != nil {
		return fmt.Errorf("http template execution failed: %+v\n", err)
	}

	pbfinal, err := imports.Process(p + ".pb.go", pbbuf.Bytes(), &imports.Options{Comments: true, FormatOnly: true})
	if err != nil {
		fmt.Printf("pb template prettification failed: %+v\n", err)
		pbfinal = pbbuf.Bytes()
	}

	httpfinal, err := imports.Process(p + ".http.go", httpbuf.Bytes(), &imports.Options{Comments: true, FormatOnly: true})
	if err != nil {
		fmt.Printf("http template prettification failed: %+v\n", err)
		httpfinal = httpbuf.Bytes()
	}

	generalfinal, err := imports.Process(p + ".go", generalbuf.Bytes(), &imports.Options{Comments: true, FormatOnly: true})
	if err != nil {
		fmt.Printf("general template prettification failed: %+v\n", err)
		httpfinal = httpbuf.Bytes()
	}

	if err := ioutil.WriteFile(p+".pb.go", pbfinal, 0644); err != nil {
		return fmt.Errorf("file write failed: %+v\n", err)
	}

	if err := ioutil.WriteFile(p+".http.go", httpfinal, 0644); err != nil {
		return fmt.Errorf("file write failed: %+v\n", err)
	}

	if err := ioutil.WriteFile(p+".go", generalfinal, 0644); err != nil {
		return fmt.Errorf("file write failed: %+v\n", err)
	}

	return nil
}
