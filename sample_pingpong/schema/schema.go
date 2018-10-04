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

	if err := (rpckit2.GoGenerator{
		Protocol:    server,
		PackageName: "main",
	}.Generate("../schema.generated.go")); err != nil {
		fmt.Fprintf(os.Stderr, "err: %+v\n", err)
	}
}
