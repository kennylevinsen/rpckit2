package rpckit2

import (
	"fmt"
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

type PropertyType int

const (
	String PropertyType = iota
	Bool
	Int64
	Int
	Float
	Double
)

func (t PropertyType) String() string {
	switch t {
	case String:
		return "String"
	case Bool:
		return "Bool"
	case Int64:
		return "Int64"
	case Int:
		return "Int"
	case Float:
		return "Float"
	case Double:
		return "Double"
	default:
		panic("unknown property type")
	}
}

func (t PropertyType) GoType() string {
	switch t {
	case String:
		return "string"
	case Bool:
		return "bool"
	case Int64:
		return "int64"
	case Int:
		return "int64"
	case Float:
		return "float32"
	case Double:
		return "float64"
	default:
		panic("unknown property type")
	}
}

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
		if m.Name == method.Name {
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
