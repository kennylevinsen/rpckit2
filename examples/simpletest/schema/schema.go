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
		ID: 1, Name: "SimpleTest",
		Description: "The simplest of tests",
		Input: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.String(), Name: "username"},
			rpckit2.Property{ID: 2, T: rpckit2.String(), Name: "password"},
		},
		Output: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.Bool(), Name: "success"},
			rpckit2.Property{ID: 2, T: rpckit2.Bool(), Name: "sortasuccess"},
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
}
