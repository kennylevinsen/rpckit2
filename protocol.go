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
	objects []Object
	methods []Method
}

func (p *Protocol) AddMethod(method Method) {
	verifMethod(p, method)
	p.methods = append(p.methods, method)
}

type Object struct {
	Name    string
	Summary string
}

type Enum struct {
	Name    string
	Summary string
}

type Method struct {
	Name        string
	ID          int64
	Description string
	Notes       string
	Input       []Property
	Output      []Property
}

//
// The PropertyType code is a big ugly, with many "IsXYZ" and accessor methods.
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
	StructName() string
	Fields() []Property
}

type simpleType struct {
	rpckitType string
	golangType string
}
func (s *simpleType) String() string { return s.rpckitType }
func (s *simpleType) GoType() string { return s.golangType }
func (simpleType) IsArray() bool { return false }
func (simpleType) IsMap() bool { return false }
func (simpleType) IsStruct() bool { return false }
func (simpleType) InnerValue() PropertyType { return nil }
func (simpleType) InnerKey() PropertyType { return nil }
func (simpleType) StructName() string { return "" }
func (simpleType) Fields() []Property { return nil }

func newSimpleType(rpckittype, golangtype string) *simpleType {
	return &simpleType{
		rpckitType: rpckittype,
		golangType: golangtype,
	}
}

func String() PropertyType { return newSimpleType("String", "string") }
func Bool() PropertyType { return newSimpleType("Bool", "bool") }
func Int64() PropertyType { return newSimpleType("Int64", "int64") }
func Int() PropertyType { return newSimpleType("Int", "int64") }
func Float() PropertyType { return newSimpleType("Float", "float32") }
func Double() PropertyType { return newSimpleType("Double", "float64") }
func Bytes() PropertyType { return newSimpleType("Bytes", "[]byte") }

type arrayType struct {
	inner PropertyType
}
func Array(T PropertyType) PropertyType { return &arrayType{inner: T} }
func (arrayType) String() string { return "Array" }
func (t *arrayType) GoType() string { return "[]" + t.inner.GoType() }
func (arrayType) IsArray() bool { return true }
func (arrayType) IsMap() bool { return false }
func (arrayType) IsStruct() bool { return false }
func (t *arrayType) InnerValue() PropertyType { return t.inner }
func (arrayType) InnerKey() PropertyType { return nil }
func (arrayType) StructName() string { return "" }
func (arrayType) Fields() []Property { return nil }

type mapType struct {
	key PropertyType
	value PropertyType
}
func Map(K, V PropertyType) PropertyType { return &mapType{key: K, value: V} }
func (mapType) String() string { return "Map" }
func (t *mapType) GoType() string {
	return "map[" + t.key.GoType() + "]" + t.value.GoType()
}
func (mapType) IsArray() bool { return false }
func (mapType) IsMap() bool { return true }
func (mapType) IsStruct() bool { return false }
func (t *mapType) InnerValue() PropertyType { return t.value }
func (t *mapType) InnerKey() PropertyType { return t.key }
func (mapType) StructName() string { return "" }
func (mapType) Fields() []Property { return nil }

type StructField struct {
	Key string
	Value PropertyType
}

type structType struct {
	name string
	fields []Property
}
func Struct(name string, fields []Property) PropertyType {
	return &structType{name: name, fields: fields}
}
func (structType) String() string { return "Struct" }
func (t *structType) GoType() string {
	return strings.Title(t.name)
}

func (structType) IsArray() bool { return false }
func (structType) IsMap() bool { return false }
func (structType) IsStruct() bool { return true }
func (structType) InnerValue() PropertyType { return nil }
func (structType) InnerKey() PropertyType { return nil }
func (t *structType) StructName() string { return t.name }
func (t *structType) Fields() []Property { return t.fields }

type Property struct {
	T    PropertyType
	ID   int64
	Name string
}

func panicf(msg string, args ...interface{}) {
	panic(fmt.Sprintf(msg, args...))
}

func verifMethod(p *Protocol, method Method) {
	verifyID(method.ID)
	verifyProperties(method.Input)
	verifyProperties(method.Output)
	for _, m := range p.methods {
		if m.ID == method.ID {
			panicf("method %v is being registered for id %v which is already in use", method.Name, method.ID)
		}
		if strings.ToLower(m.Name) == strings.ToLower(method.Name) {
			panicf("method name %v is already in use", method.Name)
		}
	}
}

func verifyProperties(properties []Property) {
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
		names[prop.Name] = true
		ids[prop.ID] = true
	}
}

func verifyID(id int64) {
	if id < 0 {
		panicf("ID must be a positive integer larger than zero")
	}
}
