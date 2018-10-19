package rpckit2

import (
	"fmt"
	"strings"
)

func NewProtocol(name string, id uint64) *Protocol {
	return &Protocol{name: name, id: id}
}

type Protocol struct {
	name    string
	id      uint64
	methods []Method
	structs []Struct
}

func (p *Protocol) AddMethod(method Method) {
	verifyMethod(p, method)
	p.methods = append(p.methods, method)
}

func (p *Protocol) AddStruct(s Struct) {
	verifyStruct(p, s)
	p.structs = append(p.structs, s)
}

type Method struct {
	Name        string
	ID          int64
	Description string
	Notes       string
	Input       []Property
	Output      []Property
}

type Struct struct {
	Name        string
	Description string
	Notes       string
	Fields      []Property
}

//
// The PropertyType code is a bit ugly, with many "IsXYZ" and accessor methods.
// This would indeed have been much prettier with a simple outer interface and
// type assertions, but these types are only accessed within the limited
// confines of a text/template context, which cannot do fancy things like type
// assertions.
//
// The best idea I've had so far, is to move all the "Is" into template
// functions, but I can't really see it helping.
//

type PropertyType interface {
	String() string
	GoType() string
	IsArray() bool
	IsMap() bool
	IsStruct() bool
	InnerValue() PropertyType
	InnerKey() PropertyType
}

type simpleType struct {
	rpckitType string
	golangType string
}

func (s *simpleType) String() string        { return s.rpckitType }
func (s *simpleType) GoType() string        { return s.golangType }
func (simpleType) IsArray() bool            { return false }
func (simpleType) IsMap() bool              { return false }
func (simpleType) IsStruct() bool           { return false }
func (simpleType) InnerValue() PropertyType { return nil }
func (simpleType) InnerKey() PropertyType   { return nil }

func newSimpleType(rpckittype, golangtype string) *simpleType {
	return &simpleType{
		rpckitType: rpckittype,
		golangType: golangtype,
	}
}

func String() PropertyType { return newSimpleType("String", "string") }
func Bool() PropertyType   { return newSimpleType("Bool", "bool") }
func Int64() PropertyType  { return newSimpleType("Int64", "int64") }
func Int() PropertyType    { return newSimpleType("Int", "int64") }
func Float() PropertyType  { return newSimpleType("Float", "float32") }
func Double() PropertyType { return newSimpleType("Double", "float64") }
func Bytes() PropertyType  { return newSimpleType("Bytes", "[]byte") }

type arrayType struct {
	inner PropertyType
}

func Array(T PropertyType) PropertyType       { return &arrayType{inner: T} }
func (arrayType) String() string              { return "Array" }
func (t *arrayType) GoType() string           { return "[]" + t.inner.GoType() }
func (arrayType) IsArray() bool               { return true }
func (arrayType) IsMap() bool                 { return false }
func (arrayType) IsStruct() bool              { return false }
func (t *arrayType) InnerValue() PropertyType { return t.inner }
func (arrayType) InnerKey() PropertyType      { return nil }

type mapType struct {
	key   PropertyType
	value PropertyType
}

func Map(K, V PropertyType) PropertyType    { return &mapType{key: K, value: V} }
func (mapType) String() string              { return "Map" }
func (mapType) IsArray() bool               { return false }
func (mapType) IsMap() bool                 { return true }
func (mapType) IsStruct() bool              { return false }
func (t *mapType) InnerValue() PropertyType { return t.value }
func (t *mapType) InnerKey() PropertyType   { return t.key }
func (t *mapType) GoType() string {
	return "map[" + t.key.GoType() + "]" + t.value.GoType()
}

type StructField struct {
	Key   string
	Value PropertyType
}

type structType struct {
	name string
}

func StructName(name string) PropertyType {
	return &structType{name: name}
}
func (structType) String() string           { return "Struct" }
func (structType) IsArray() bool            { return false }
func (structType) IsMap() bool              { return false }
func (structType) IsStruct() bool           { return true }
func (structType) InnerValue() PropertyType { return nil }
func (structType) InnerKey() PropertyType   { return nil }
func (t *structType) GoType() string {
	return strings.Title(t.name)
}

type Property struct {
	T           PropertyType
	ID          int64
	Name        string
	Description string
}

func panicf(msg string, args ...interface{}) {
	panic(fmt.Sprintf(msg, args...))
}

func verifyMethod(p *Protocol, method Method) {
	verifyID(method.ID)
	verifyProperties(p, method.Input)
	verifyProperties(p, method.Output)
	for _, m := range p.methods {
		if m.ID == method.ID {
			panicf("method %v is being registered for id %v which is already in use", method.Name, method.ID)
		}
		if strings.ToLower(m.Name) == strings.ToLower(method.Name) {
			panicf("method name %v is already in use", method.Name)
		}
	}
}

func verifyStruct(p *Protocol, s Struct) {
	verifyProperties(p, s.Fields)
	for _, m := range p.structs {
		if strings.ToLower(m.Name) == strings.ToLower(s.Name) {
			panicf("struct name %v is already in use", s.Name)
		}
	}
}

func verifyProperties(p *Protocol, properties []Property) {
	if properties == nil {
		return
	}

	names := make(map[string]bool)
	ids := make(map[int64]bool)
	for _, prop := range properties {
		verifyID(prop.ID)
		if _, found := names[prop.Name]; found {
			panicf("the property named %v is already in use", prop.Name)
		}
		if _, found := ids[prop.ID]; found {
			panicf("the property id %v is already in use", prop.ID)
		}
		if prop.T.IsStruct() {
			var found bool
			for _, s := range p.structs {
				if strings.Title(s.Name) == prop.T.GoType() {
					found = true
					break
				}
			}
			if !found {
				panicf("the struct %s has not been defined", prop.T.GoType())
			}
		}
		names[prop.Name] = true
		ids[prop.ID] = true
	}
}

func verifyID(id int64) {
	if id < 0 {
		panicf("ID must be a positive integer larger than zero")
	}
}
