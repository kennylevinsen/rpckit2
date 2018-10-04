package main

import (
	"fmt"
	"os"

	"gitlab.com/exashape/rpckit2"
)

//go:generate go run schema.go

func main() {
	//fmt.Println("hello")
	server := rpckit2.NewProtocol("pingpong")
	//server.AddError(101, "not allowed")
	server.AddMethod(rpckit2.Method{
		ID: 1, Name: "Authenticate",
		Description: "Connect",
		Input: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.String, Name: "username"},
			rpckit2.Property{ID: 2, T: rpckit2.String, Name: "password"},
		},
		Output: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.Bool, Name: "success"},
		},
	})
	server.AddMethod(rpckit2.Method{
		ID: 2, Name: "PingWithReply",
		Description: "Echo",
		Input: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.String, Name: "name"},
		},
		Output: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.String, Name: "greeting"},
		},
	})
	server.AddMethod(rpckit2.Method{
		ID: 3, Name: "TestMethod",
		Description: "Echo",
		Input: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.String, Name: "string"},
			rpckit2.Property{ID: 2, T: rpckit2.Bool, Name: "bool"},
			rpckit2.Property{ID: 3, T: rpckit2.Int64, Name: "int64"},
			rpckit2.Property{ID: 4, T: rpckit2.Int, Name: "int"},
			rpckit2.Property{ID: 5, T: rpckit2.Float, Name: "float"},
			rpckit2.Property{ID: 6, T: rpckit2.Double, Name: "double"},
		},
		Output: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.Bool, Name: "success"},
		},
	})

	if err := (rpckit2.GoGenerator{
		Protocol:    server,
		PackageName: "main",
	}.Generate("../schema.generated.go")); err != nil {
		fmt.Fprintf(os.Stderr, "err: %+v\n", err)
	}
}
