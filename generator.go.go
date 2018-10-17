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
	"unicode"
	"log"

	"golang.org/x/tools/imports"
)

//go:generate go-bindata --pkg rpckit2 -o templates.go templates/...

var template_deps = []string{
	"go-pb/boilerplate.go.tmpl",
	"go-pb/property_to_wiretype.go.tmpl",
	"go-pb/marshalsimple.go.tmpl",
	"go-pb/marshalarray.go.tmpl",
	"go-pb/marshalmap.go.tmpl",
	"go-pb/marshalstruct.go.tmpl",
	"go-pb/marshal.go.tmpl",
	"go-pb/unmarshalsimple.go.tmpl",
	"go-pb/unmarshalarray.go.tmpl",
	"go-pb/unmarshalmap.go.tmpl",
	"go-pb/unmarshalstruct.go.tmpl",
	"go-pb/unmarshal.go.tmpl",
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

	"go/types.go.tmpl",
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
	exists := make(map[string]bool)

	funcs := template.FuncMap{
		"log": func (formatter string, v ...interface{}) string {
			log.Printf(formatter, v...)
			return ""
		},
		"ifnotexists": func(name string) bool {
			if exists[name] {
				return false
			}
			exists[name] = true
			return true
		},
		"format": func (formatter string, v ...interface{}) string {
			return fmt.Sprintf(formatter, v...)
		},
		"error": func(s string) error {
			return errors.New(s)
		},
		"capitalize": func(s string) string {
			var b strings.Builder
			b.Grow(len(s))
			for idx, r := range s {
				if idx == 0 {
					b.WriteRune(unicode.ToTitle(r))
					continue
				}
				b.WriteRune(r)
			}
			return b.String()
		},
		"upper":       strings.ToTitle,
		"lower":       strings.ToLower,
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

	for _, v := range []string{"pb", "http", ""} {
		filepath := p
		templatepath := "/rpckit.go.tmpl"
		if v == "" {
			filepath += ".go"
			templatepath = "go" + templatepath
		} else {
			filepath += "." + v + ".go"
			templatepath = "go-" + v + templatepath
		}

		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, templatepath, ctx); err != nil {
			return fmt.Errorf("%s template execution failed: %+v", v, err)
		}

		final, err := imports.Process(filepath, buf.Bytes(), &imports.Options{Comments: true, FormatOnly: true})
		if err != nil {
			return fmt.Errorf("%s template prettification failed: %+v", v, err)
		}

		if err := ioutil.WriteFile(filepath, final, 0644); err != nil {
			return fmt.Errorf("file write failed: %+v", err)
		}
	}

	return nil
}
