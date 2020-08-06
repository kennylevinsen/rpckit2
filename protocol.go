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

type MethodInputOptions struct{}
type MethodOutputOptions struct {
	OmitEmpty bool // TODO: json encoding flag and only supported on http.
}

type StructFieldOptions struct {
	Getter    bool
	Setter    bool
	OmitEmpty bool // TODO: json encoding flag and only supported on http.
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
	SwiftType() string
	SwiftDefault() string
	IsArray() bool
	IsMap() bool
	IsStruct() bool
	IsMarshalled() bool
	InnerValue() PropertyType
	InnerKey() PropertyType
}

type simpleType struct {
	rpckitType string
	golangType string
	swiftType string
	swiftDefault string
}

func (s *simpleType) String() string        { return s.rpckitType }
func (s *simpleType) GoType() string        { return s.golangType }
func (s *simpleType) SwiftType() string     { return s.swiftType }
func (s *simpleType) SwiftDefault() string  { return s.swiftDefault }
func (simpleType) IsArray() bool            { return false }
func (simpleType) IsMap() bool              { return false }
func (simpleType) IsStruct() bool           { return false }
func (simpleType) IsMarshalled() bool       { return false }
func (simpleType) InnerValue() PropertyType { return nil }
func (simpleType) InnerKey() PropertyType   { return nil }

func newSimpleType(rpckittype, golangtype, swiftType, swiftDefault string) *simpleType {
	return &simpleType{
		rpckitType: rpckittype,
		golangType: golangtype,
		swiftType: swiftType,
		swiftDefault: swiftDefault,
	}
}

type marshalledType struct {
	rpckitType string
	golangType string
	swiftType string
	swiftDefault string
	_import    string
}

func (s *marshalledType) String() string        { return s.rpckitType }
func (s *marshalledType) GoType() string        { return s.golangType }
func (s *marshalledType) SwiftType() string     { return s.swiftType }
func (s *marshalledType) SwiftDefault() string  { return s.swiftDefault }
func (s *marshalledType) Import() string        { return s._import }
func (marshalledType) IsArray() bool            { return false }
func (marshalledType) IsMap() bool              { return false }
func (marshalledType) IsStruct() bool           { return false }
func (marshalledType) IsMarshalled() bool       { return true }
func (marshalledType) InnerValue() PropertyType { return nil }
func (marshalledType) InnerKey() PropertyType   { return nil }

func newMarshalledType(rpckittype, golangtype, swiftType, swiftDefault, _import string) *marshalledType {
	return &marshalledType{
		rpckitType: rpckittype,
		golangType: golangtype,
		swiftType: swiftType,
		swiftDefault: swiftDefault,
		_import:    _import,
	}
}

func String() PropertyType   { return newSimpleType("String", "string", "String", "\"\"") }
func Bool() PropertyType     { return newSimpleType("Bool", "bool", "Bool", "false") }
func Int64() PropertyType    { return newSimpleType("Int64", "int64", "Int64", "0") }
func Int() PropertyType      { return newSimpleType("Int", "int64", "Int64", "0") }
func Float() PropertyType    { return newSimpleType("Float", "float32", "Float", "0.0") }
func Double() PropertyType   { return newSimpleType("Double", "float64", "Double", "0.0") }
func Bytes() PropertyType    { return newSimpleType("Bytes", "[]byte", "ArraySlice<UInt8>", "[]") }
func DateTime() PropertyType { return newMarshalledType("DateTime", "time.Time", "", "","time") }
func UUID() PropertyType     { return newMarshalledType("UUID", "uuid.UUID", "", "", "github.com/satori/go.uuid") }

type MarshalledProperty interface {
	Import() string
}

type arrayType struct {
	inner PropertyType
}

func Array(T PropertyType) PropertyType       { return &arrayType{inner: T} }
func (arrayType) String() string              { return "Array" }
func (t *arrayType) GoType() string           { return "[]" + t.inner.GoType() }
func (t *arrayType) SwiftType() string        { return "[" + t.inner.SwiftType() + "]"}
func (t *arrayType) SwiftDefault() string     { return "[]"}
func (arrayType) IsArray() bool               { return true }
func (arrayType) IsMap() bool                 { return false }
func (arrayType) IsStruct() bool              { return false }
func (arrayType) IsMarshalled() bool          { return false }
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
func (mapType) IsMarshalled() bool          { return false }
func (t *mapType) InnerValue() PropertyType { return t.value }
func (t *mapType) InnerKey() PropertyType   { return t.key }
func (t *mapType) GoType() string {
	return "map[" + t.key.GoType() + "]" + t.value.GoType()
}
func (t *mapType) SwiftType() string {
	return "[" + t.key.SwiftType() + " : " + t.value.SwiftType() + " ]"
}
func (t *mapType) SwiftDefault() string {
	return "[:]"
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
func (structType) IsMarshalled() bool       { return false }
func (structType) InnerValue() PropertyType { return nil }
func (structType) InnerKey() PropertyType   { return nil }
func (t *structType) GoType() string {
	return strings.Title(t.name)
}
func (t *structType) SwiftType() string {
	return strings.Title(t.name)
}

func (t *structType) SwiftDefault() string {
	return strings.Title(t.name) + "()"
}

type Property struct {
	T           PropertyType
	ID          int64
	Name        string
	Description string
	Options     interface{}
}

func panicf(msg string, args ...interface{}) {
	panic(fmt.Sprintf(msg, args...))
}

func verifyMethod(p *Protocol, method Method) {
	verifyID(method.ID)
	if method.Name == "" {
		panicf("method %d lacks a name", method.ID)
	}
	if method.Description == "" {
		panicf("method %v lacks a description", method.Name)
	}
	verifyProperties(p, method.Input, false, true)
	verifyProperties(p, method.Output, false, false)
	for _, m := range p.methods {
		if m.ID == method.ID {
			panicf("method %v is being registered for id %v which is already in use by %v", method.Name, method.ID, m.Name)
		}
		if strings.ToLower(m.Name) == strings.ToLower(method.Name) {
			panicf("method name %v is already in use", method.Name)
		}
	}
}

func verifyStruct(p *Protocol, s Struct) {
	if s.Name == "" {
		panicf("struct lacks a name")
	}
	if s.Description == "" {
		panicf("struct %v lacks a description", s.Name)
	}
	verifyProperties(p, s.Fields, true, false)
	for _, m := range p.structs {
		if strings.ToLower(m.Name) == strings.ToLower(s.Name) {
			panicf("struct name %v is already in use", s.Name)
		}
	}
}

func verifyProperties(p *Protocol, properties []Property, isStruct, isInput bool) {
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
		// if prop.T.IsStruct() {
		// 	var found bool
		// 	for _, s := range p.structs {
		// 		if strings.Title(s.Name) == prop.T.GoType() {
		// 			found = true
		// 			break
		// 		}
		// 	}
		// 	if !found {
		// 		panicf("the struct %s has not been defined", prop.T.GoType())
		// 	}
		// }

		if isStruct {
			if prop.Options != nil {
				if _, ok := prop.Options.(StructFieldOptions); !ok {
					panicf("the struct property with id %v can only recieve options of type StructFieldOptions, but recieved type %T", prop.ID, prop.Options)
				}
			}
		} else {
			if prop.Options != nil {
				if isInput {
					if _, ok := prop.Options.(MethodInputOptions); !ok {
						panicf("the input property with id %v can only recieve options of type MethodInputOptions, but recieved type %T", prop.ID, prop.Options)
					}
				} else {
					if _, ok := prop.Options.(MethodOutputOptions); !ok {
						panicf("the output property with id %v can only recieve options of type MethodInputOptions, but recieved type %T", prop.ID, prop.Options)
					}
				}
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
