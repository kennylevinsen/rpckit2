package main

import (
	"fmt"
	"os"

	"gitlab.com/exashape/rpckit2"
)

//go:generate go run schema.go

func main() {

	backend := rpckit2.NewProtocol("Backend", 1)
	/*backend.AddStruct(rpckit2.Struct{
		Name:        "ChannelInfo",
		Description: "Channel info object to contain details of channel",
		Fields: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.Int64(), Name: "ID"},
			rpckit2.Property{ID: 2, T: rpckit2.String(), Name: "Title"},
		},
	})*/
/*
	backend.AddMethod(rpckit2.Method{
		ID: 1, Name: "Authenticate",
		Description: "Authenticate using either (token or e-mail).",
		Input: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.String(), Name: "token"},
			rpckit2.Property{ID: 2, T: rpckit2.String(), Name: "email"},
		},
		Output: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.String(), Name: "token"},
			//TOOD: More details about self: avatar, username, etc...
		},
	})
	backend.AddMethod(rpckit2.Method{
		ID: 2, Name: "ListChannels",
		Description: "List my active channels",
		Input:       []rpckit2.Property{},
		Output: []rpckit2.Property{
			//TODO: an array of channelinfo(?) objects
			rpckit2.Property{ID: 1, T: rpckit2.String(), Name: "greeting"},
			//rpckit2.Property{ID: 2, T: rpckit2.Array(rpckit2.StructName("ChannelInfo")), Name: "Channels"},
		},
	})

	backend.AddMethod(rpckit2.Method{
		ID: 3, Name: "SetAvatar",
		Description: "Set the avatar for the currently authenticated user",
		Input: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.Bytes(), Name: "image"},
		},
		Output: []rpckit2.Property{
			//TODO: Self userinfo struct
		},
	})
	backend.AddMethod(rpckit2.Method{
		ID: 4, Name: "SetName",
		Description: "Set the name of the current user",
		Input: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.String(), Name: "name"},
		},
		Output: []rpckit2.Property{
			//TODO: Self userinfo struct
		},
	})
	backend.AddMethod(rpckit2.Method{
		ID: 5, Name: "RegisterNotifications",
		Description: "Register for push notifications",
		Input: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.String(), Name: "token"},
			rpckit2.Property{ID: 2, T: rpckit2.String(), Name: "name"},
			rpckit2.Property{ID: 3, T: rpckit2.String(), Name: "devicetype"},
		},
	})*/
	// TODO: Add update avatar method.
	/*server1.AddMethod(rpckit2.Method{
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
	})*/

	//fmt.Println("hello")
	server1 := rpckit2.NewProtocol("pingpong", 1)
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
		},
		Output: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.Int(), Name: "vinteger"},
			rpckit2.Property{ID: 2, T: rpckit2.Int64(), Name: "vint64"},
			rpckit2.Property{ID: 3, T: rpckit2.Float(), Name: "vfloat"},
			rpckit2.Property{ID: 4, T: rpckit2.Double(), Name: "vdouble"},
			rpckit2.Property{ID: 5, T: rpckit2.Bool(), Name: "vbool"},
			rpckit2.Property{ID: 6, T: rpckit2.String(), Name: "vstring"},
			rpckit2.Property{ID: 7, T: rpckit2.Bytes(), Name: "vbytes"},
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
		},
		Output: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.Array(rpckit2.Int()), Name: "vinteger"},
			rpckit2.Property{ID: 2, T: rpckit2.Array(rpckit2.Int64()), Name: "vint64"},
			rpckit2.Property{ID: 3, T: rpckit2.Array(rpckit2.Float()), Name: "vfloat"},
			rpckit2.Property{ID: 4, T: rpckit2.Array(rpckit2.Double()), Name: "vdouble"},
			rpckit2.Property{ID: 5, T: rpckit2.Array(rpckit2.Bool()), Name: "vbool"},
			rpckit2.Property{ID: 6, T: rpckit2.Array(rpckit2.String()), Name: "vstring"},
			rpckit2.Property{ID: 7, T: rpckit2.Array(rpckit2.Bytes()), Name: "vbytes"},
		},
	})
	server2 := rpckit2.NewProtocol("wowo", 2)

	server2.AddMethod(rpckit2.Method{
		ID: 1, Name: "SimpleTest2",
		Description: "The simplest of tests",
		Input: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.String(), Name: "token"},
			// rpckit2.Property{ID: 2, T: rpckit2.String(), Name: "email"},
		},
		Output: []rpckit2.Property{
			rpckit2.Property{ID: 1, T: rpckit2.String(), Name: "token"},
			//TOOD: More details about self: avatar, username, etc...
		},
	})

	server2.AddMethod(rpckit2.Method{
		ID: 2, Name: "SimpleTest3",
		Description: "The simplest of tests",
		Input: []rpckit2.Property{
		},
		Output: []rpckit2.Property{
		},
	})
	if err := (rpckit2.SwiftGenerator{
		Protocols: []*rpckit2.Protocol{
			server1,
			server2,
			backend,
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
