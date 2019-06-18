package rpckit2

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"path"
	"strconv"
	"strings"
	"text/template"
	"unicode"

	"golang.org/x/tools/imports"
)

//go:generate go-bindata --pkg rpckit2 -o templates.go templates/...

var template_deps = []string{
	"go-pb/boilerplate.go.tmpl",
	"go-pb/property_to_wiretype.go.tmpl",
	"go-pb/marshal_simple.go.tmpl",
	"go-pb/marshal_array.go.tmpl",
	"go-pb/marshal_map.go.tmpl",
	"go-pb/marshal_struct.go.tmpl",
	"go-pb/marshal.go.tmpl",
	"go-pb/unmarshal_simple.go.tmpl",
	"go-pb/unmarshal_array.go.tmpl",
	"go-pb/unmarshal_map.go.tmpl",
	"go-pb/unmarshal_struct.go.tmpl",
	"go-pb/unmarshal.go.tmpl",
	"go-pb/prepare_serializers.go.tmpl",
	"go-pb/serializations.go.tmpl",
	"go-pb/serialization.go.tmpl",
	"go-pb/serialization_map.go.tmpl",
	"go-pb/serialization_struct.go.tmpl",
	"go-pb/client_definitions.go.tmpl",
	"go-pb/client_methods.go.tmpl",
	"go-pb/server_definitions.go.tmpl",
	"go-pb/server_methods.go.tmpl",
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
	Structs []Struct
}

type TemplateContext struct {
	PackageName string
	Protocols   []TemplateProtocol
}

func (g GoGenerator) Generate(p string) error {
	exists := make(map[string]bool)
	counter := make(map[string]int)
	store := make(map[string]interface{})

	funcs := template.FuncMap{
		"log": func(formatter string, v ...interface{}) string {
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
		"required": func(i interface{}, d ...string) (interface{}, error) {
			if i == nil {
				return "", fmt.Errorf("required value missing")
			}
			switch x := i.(type) {
			case string:
				if x == "" {
					return "", fmt.Errorf("required value empty")
				}
			}
			return i, nil
		},
		"counter": func(name string) int {
			cnt := counter[name]
			counter[name] = cnt + 1
			return cnt
		},
		"store": func(name string, v interface{}) string {
			store[name] = v
			return ""
		},
		"retrieve": func(name string) interface{} {
			return store[name]
		},
		"format": func(formatter string, v ...interface{}) string {
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
					b.WriteRune(unicode.ToUpper(r))
					continue
				}
				b.WriteRune(r)
			}
			return b.String()
		},
		"camelize": func(s string) string {
			upperCase := 0
			isFullUpper := true
			for _, r := range s {
				if !unicode.IsUpper(r) {
					isFullUpper = false
					break
				}
				upperCase += 1
			}

			// If all letters are upper-case, we assume it is an abbreviation and lower-case it.
			if isFullUpper {
				return strings.ToLower(s)
			}

			var b strings.Builder
			b.Grow(len(s))
			for idx, r := range s {
				if idx == 0 || idx < upperCase - 1 {
					b.WriteRune(unicode.ToLower(r))
					continue
				}
				b.WriteRune(r)
			}
			return b.String()
		},
		"upper":       strings.ToUpper,
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
			Structs: v.structs,
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
			fmt.Printf("%s template prettification failed: %+v\n", v, err)
			// return fmt.Errorf("%s template prettification failed: %+v", v, err)
			final = buf.Bytes()
		}

		if err := ioutil.WriteFile(filepath, final, 0644); err != nil {
			return fmt.Errorf("file write failed: %+v", err)
		}
	}

	return nil
}
