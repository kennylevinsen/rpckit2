package main

import (
	"fmt"
	"os"

	"gitlab.com/exashape/rpckit2"
)

//go:generate go run schema.go

func main() {
	//fmt.Println("hello")
	server1 := rpckit2.NewProtocol("pingpong", 1)
	server1.AddStruct(rpckit2.Struct{
		Name:        "ssss",
		Description: "A struct",
		Fields: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.String(), Name: "wee", Description: "WAAAAH"},
		},
	})

	//server.AddError(101, "not allowed")
	server1.AddMethod(rpckit2.Method{
		ID: 1, Name: "SimpleTest",
		Description: "The simplest of tests",
		Input: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.Int(), Name: "vinteger"},
			rpckit2.Property{ID: 2, T: rpckit2.Int64(), Name: "vint64"},
			rpckit2.Property{ID: 3, T: rpckit2.Float(), Name: "vfloat"},
			rpckit2.Property{ID: 4, T: rpckit2.Double(), Name: "vdouble"},
			rpckit2.Property{ID: 5, T: rpckit2.Bool(), Name: "vbool"},
			rpckit2.Property{ID: 6, T: rpckit2.String(), Name: "vstring"},
			rpckit2.Property{ID: 7, T: rpckit2.Bytes(), Name: "vbytes"},
			rpckit2.Property{ID: 8, T: rpckit2.StructName("ssss"), Name: "vstruct"},
		},
		Output: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.Int(), Name: "vinteger"},
			rpckit2.Property{ID: 2, T: rpckit2.Int64(), Name: "vint64"},
			rpckit2.Property{ID: 3, T: rpckit2.Float(), Name: "vfloat"},
			rpckit2.Property{ID: 4, T: rpckit2.Double(), Name: "vdouble"},
			rpckit2.Property{ID: 5, T: rpckit2.Bool(), Name: "vbool"},
			rpckit2.Property{ID: 6, T: rpckit2.String(), Name: "vstring"},
			rpckit2.Property{ID: 7, T: rpckit2.Bytes(), Name: "vbytes"},
			rpckit2.Property{ID: 8, T: rpckit2.StructName("ssss"), Name: "vstruct"},
		},
	})

	server1.AddMethod(rpckit2.Method{
		ID: 2, Name: "ArrayTest",
		Description: "The simplest of tests, but with arrays",
		Input: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.Array(rpckit2.Int()), Name: "vinteger"},
			rpckit2.Property{ID: 2, T: rpckit2.Array(rpckit2.Int64()), Name: "vint64"},
			rpckit2.Property{ID: 3, T: rpckit2.Array(rpckit2.Float()), Name: "vfloat"},
			rpckit2.Property{ID: 4, T: rpckit2.Array(rpckit2.Double()), Name: "vdouble"},
			rpckit2.Property{ID: 5, T: rpckit2.Array(rpckit2.Bool()), Name: "vbool"},
			rpckit2.Property{ID: 6, T: rpckit2.Array(rpckit2.String()), Name: "vstring"},
			rpckit2.Property{ID: 7, T: rpckit2.Array(rpckit2.Bytes()), Name: "vbytes"},
			rpckit2.Property{ID: 8, T: rpckit2.Array(rpckit2.StructName("ssss")), Name: "vstruct"},
		},
		Output: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.Array(rpckit2.Int()), Name: "vinteger"},
			rpckit2.Property{ID: 2, T: rpckit2.Array(rpckit2.Int64()), Name: "vint64"},
			rpckit2.Property{ID: 3, T: rpckit2.Array(rpckit2.Float()), Name: "vfloat"},
			rpckit2.Property{ID: 4, T: rpckit2.Array(rpckit2.Double()), Name: "vdouble"},
			rpckit2.Property{ID: 5, T: rpckit2.Array(rpckit2.Bool()), Name: "vbool"},
			rpckit2.Property{ID: 6, T: rpckit2.Array(rpckit2.String()), Name: "vstring"},
			rpckit2.Property{ID: 7, T: rpckit2.Array(rpckit2.Bytes()), Name: "vbytes"},
			rpckit2.Property{ID: 8, T: rpckit2.Array(rpckit2.StructName("ssss")), Name: "vstruct"},
		},
	})

	if err := (rpckit2.SwiftGenerator{
		Protocols: []*rpckit2.Protocol{
			server1,
		},
		PackageName: "main",
	}.Generate("../schema.generated")); err != nil {
		fmt.Fprintf(os.Stderr, "err: %+v\n", err)
	}

	if err := (rpckit2.GoGenerator{
		Protocols: []*rpckit2.Protocol{
			server1,
		},
		PackageName: "main",
	}.Generate("../schema.generated")); err != nil {
		fmt.Fprintf(os.Stderr, "err: %+v\n", err)
	}
}
