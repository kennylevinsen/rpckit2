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

type PropertyType interface {
	String() string
	GoType() string
	IsArray() bool
	IsMap() bool
	InnerValue() PropertyType
	InnerKey() PropertyType
}

type stringType struct{}
func String() PropertyType { return stringType{} }
func (stringType) String() string { return "String" }
func (stringType) GoType() string { return "string" }
func (stringType) IsArray() bool { return false }
func (stringType) IsMap() bool { return false }
func (stringType) InnerValue() PropertyType { return nil }
func (stringType) InnerKey() PropertyType { return nil }

type boolType struct {}
func Bool() PropertyType { return boolType{} }
func (boolType) String() string { return "Bool" }
func (boolType) GoType() string { return "bool" }
func (boolType) IsArray() bool { return false }
func (boolType) IsMap() bool { return false }
func (boolType) InnerValue() PropertyType { return nil }
func (boolType) InnerKey() PropertyType { return nil }

type int64Type struct {}
func Int64() PropertyType { return int64Type{} }
func (int64Type) String() string { return "Int64" }
func (int64Type) GoType() string { return "int64" }
func (int64Type) IsArray() bool { return false }
func (int64Type) IsMap() bool { return false }
func (int64Type) InnerValue() PropertyType { return nil }
func (int64Type) InnerKey() PropertyType { return nil }

type intType struct {}
func Int() PropertyType { return intType{} }
func (intType) String() string { return "Int" }
func (intType) GoType() string { return "int64" }
func (intType) IsArray() bool { return false }
func (intType) IsMap() bool { return false }
func (intType) InnerValue() PropertyType { return nil }
func (intType) InnerKey() PropertyType { return nil }

type floatType struct {}
func Float() PropertyType { return floatType{} }
func (floatType) String() string { return "Float" }
func (floatType) GoType() string { return "float32" }
func (floatType) IsArray() bool { return false }
func (floatType) IsMap() bool { return false }
func (floatType) InnerValue() PropertyType { return nil }
func (floatType) InnerKey() PropertyType { return nil }

type doubleType struct {}
func Double() PropertyType { return doubleType{} }
func (doubleType) String() string { return "Double" }
func (doubleType) GoType() string { return "float64" }
func (doubleType) IsArray() bool { return false }
func (doubleType) IsMap() bool { return false }
func (doubleType) InnerValue() PropertyType { return nil }
func (doubleType) InnerKey() PropertyType { return nil }

type bytesType struct{}
func Bytes() PropertyType { return bytesType{} }
func (bytesType) String() string { return "Bytes" }
func (bytesType) GoType() string { return "[]byte" }
func (bytesType) IsArray() bool { return false }
func (bytesType) IsMap() bool { return false }
func (bytesType) InnerValue() PropertyType { return nil }
func (bytesType) InnerKey() PropertyType { return nil }

type arrayType struct {
	inner PropertyType
}
func Array(T PropertyType) PropertyType { return &arrayType{inner: T} }
func (arrayType) String() string { return "Array" }
func (t *arrayType) GoType() string { return "[]" + t.inner.GoType() }
func (arrayType) IsArray() bool { return true }
func (arrayType) IsMap() bool { return false }
func (t *arrayType) InnerValue() PropertyType { return t.inner }
func (arrayType) InnerKey() PropertyType { return nil }

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
func (t *mapType) InnerValue() PropertyType { return t.value }
func (t *mapType) InnerKey() PropertyType { return t.key }

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
