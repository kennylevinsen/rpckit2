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
	//server.AddError(101, "not allowed")
	server1.AddMethod(rpckit2.Method{
		ID: 1, Name: "Authenticate",
		Description: "Authenticate using username and password",
		Input: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.String(), Name: "username"},
			rpckit2.Property{ID: 2, T: rpckit2.String(), Name: "password"},
		},
		Output: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.Bool(), Name: "success"},
		},
	})
	server1.AddMethod(rpckit2.Method{
		ID: 2, Name: "PingWithReply",
		Description: "PingWithReply replies with a greeting based on the provided name",
		Input: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.String(), Name: "name"},
		},
		Output: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.String(), Name: "greeting"},
		},
	})
	server1.AddMethod(rpckit2.Method{
		ID: 3, Name: "TestMethod",
		Description: "TestMethod is a simple type test",
		Input: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.String(), Name: "string"},
			rpckit2.Property{ID: 2, T: rpckit2.Bool(), Name: "bool"},
			rpckit2.Property{ID: 3, T: rpckit2.Int64(), Name: "int64"},
			rpckit2.Property{ID: 4, T: rpckit2.Int(), Name: "int"},
			rpckit2.Property{ID: 5, T: rpckit2.Float(), Name: "float"},
			rpckit2.Property{ID: 6, T: rpckit2.Double(), Name: "double"},
			rpckit2.Property{ID: 7, T: rpckit2.DateTime(), Name: "datetime"},
		},
		Output: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.Bool(), Name: "success"},
		},
	})

	server2 := rpckit2.NewProtocol("echo", 2)
	server2.AddStruct(rpckit2.Struct{
		Name:        "echoThing",
		Description: "Echo thing",
		Fields: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.String(), Name: "wee", Description: "WAAAAH"},
			rpckit2.Property{ID: 2, T: rpckit2.String(), Name: "woo", Description: "woo describes the woo factor"},
			rpckit2.Property{ID: 3, T: rpckit2.Map(rpckit2.String(), rpckit2.Int()), Name: "stuff", Description: "weee"},
			rpckit2.Property{ID: 4, T: rpckit2.DateTime(), Name: "anothertime", Description: "My wao time"},
			rpckit2.Property{ID: 5, T: rpckit2.Map(rpckit2.DateTime(), rpckit2.DateTime()), Name: "mydatetimemap", Description: "datetime mapsimap"},
		},
	})

	server2.AddMethod(rpckit2.Method{
		ID: 1, Name: "Echo",
		Description: "Echo is yet another type test",
		Input: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.String(), Name: "input"},
			rpckit2.Property{ID: 2, T: rpckit2.Array(rpckit2.String()), Name: "names"},
			rpckit2.Property{ID: 3, T: rpckit2.Map(rpckit2.String(), rpckit2.Map(rpckit2.String(), rpckit2.Int())), Name: "values"},
			rpckit2.Property{ID: 4, T: rpckit2.Map(rpckit2.String(), rpckit2.Int()), Name: "values2"},
			rpckit2.Property{ID: 5, T: rpckit2.StructName("echoThing"), Name: "something"},
			rpckit2.Property{ID: 6, T: rpckit2.DateTime(), Name: "mytime", Description: "My wao time"},
		},
		Output: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.String(), Name: "output"},
		},
	})
	server2.AddMethod(rpckit2.Method{
		ID: 2, Name: "Ping",
		Description: "Ping is a simple no-input test",
		Input:       []rpckit2.Property{},
		Output: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.String(), Name: "output"},
		},
	})
	server2.AddMethod(rpckit2.Method{
		ID: 3, Name: "ByteTest",
		Description: "ByteTest is a byte test",
		Input: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.Bytes(), Name: "input"},
		},
		Output: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.Bytes(), Name: "output"},
		},
	})

	if err := (rpckit2.GoGenerator{
		Protocols: []*rpckit2.Protocol{
			server1,
			server2,
		},
		PackageName: "main",
	}.Generate("../schema.generated")); err != nil {
		fmt.Fprintf(os.Stderr, "err: %+v\n", err)
	}
}
