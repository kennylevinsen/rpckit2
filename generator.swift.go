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
)

//go:generate go-bindata --pkg rpckit2 -o templates.go templates/...

var swift_template_deps = []string{
	"swift-pb.tmpl",
}

type SwiftGenerator struct {
	PackageName string
	Protocols   []*Protocol
}

type SwiftTemplateProtocol struct {
	Name    string
	ID      uint64
	Methods []Method
	Structs []Struct
}

type SwiftTemplateContext struct {
	PackageName string
	Protocols   []SwiftTemplateProtocol
	Imports     map[string]struct{}
}

func (g SwiftGenerator) Generate(p string) error {
	exists := make(map[string]bool)
	counter := make(map[string]int)
	store := make(map[string]interface{})

	funcs := template.FuncMap{
		"add": func(a, b int) int {
			return a + b
		},
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
				if idx == 0 || idx < upperCase-1 {
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
	for _, name := range swift_template_deps {
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

	ctx := SwiftTemplateContext{
		PackageName: g.PackageName,
		Imports:     make(map[string]struct{}),
	}

	for _, v := range g.Protocols {
		ctx.Protocols = append(ctx.Protocols, SwiftTemplateProtocol{
			Name:    v.name,
			ID:      v.id,
			Methods: v.methods,
			Structs: v.structs,
		})
		for _, s := range v.structs {
			for _, f := range s.Fields {
				if p, ok := f.T.(MarshalledProperty); ok {
					ctx.Imports[p.Import()] = struct{}{}
				}
			}
		}
		for _, m := range v.methods {
			for _, f := range m.Input {
				if p, ok := f.T.(MarshalledProperty); ok {
					ctx.Imports[p.Import()] = struct{}{}
				}
			}
			for _, f := range m.Output {
				if p, ok := f.T.(MarshalledProperty); ok {
					ctx.Imports[p.Import()] = struct{}{}
				}
			}
		}
	}

	for _, v := range []string{"pb"} {
		filepath := p
		var templatepath string
		if v == "" {
			filepath += ".swift"
			templatepath = "swift.tmpl"
		} else {
			filepath += "." + v + ".swift"
			templatepath = "swift-" + v + ".tmpl"
		}

		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, templatepath, ctx); err != nil {
			return fmt.Errorf("%s template execution failed: %+v", v, err)
		}

		if err := ioutil.WriteFile(filepath, buf.Bytes(), 0644); err != nil {
			return fmt.Errorf("file write failed: %+v", err)
		}
	}

	return nil
}
