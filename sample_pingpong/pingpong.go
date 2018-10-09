package main

import (
	"fmt"
	"net"
	"context"
	"net/http"
)

func main() {
	// listen for network connections
	l1, err := net.Listen("tcp", ":9999")
	if err != nil {
		panic(err)
	}
	l2, err := net.Listen("tcp", ":9998")
	if err != nil {
		panic(err)
	}
	go func() {
		for {
			conn, err := l1.Accept()
			if err != nil {
				panic(err)
			}

			NewRPCConnection(&RPCOptions{
				Conn: conn,
				Handlers: []RPCServer{
					RPCPingpongHandler(&server{}),
					RPCEchoHandler(&echoServer{}),
				},
				ConnectHook: func(c *RPCConnection) error {
					fmt.Printf("Server: user connected\n")
					return nil
				},
				DisconnectHook: func(c *RPCConnection, err RPCError) {
					fmt.Printf("Server: user disconnected\n")
				},
			})
		}
	}()

	go func() {
		mux := http.NewServeMux()

		h1 := HTTPEchoHandler(&echoServer{})
		h2 := HTTPPingpongHandler(&server{})

		h1.RegisterToMux(mux)
		h2.RegisterToMux(mux)

		http.Serve(l2, mux)
	}()

	ctx := context.Background()

	NewRPCConnection(&RPCOptions{
		Dialer: func() (net.Conn, error) {
			return net.Dial("tcp", "127.0.0.1:9999")
		},
		ConnectHook: func(c *RPCConnection) error {
			fmt.Printf("Client: connected to server\n")

			serverClient := NewRPCPingpongClient(c)
			echoClient := NewRPCEchoClient(c)

			success, err := serverClient.Authenticate(ctx, "batman", "robin")
			if err != nil {
				panic(fmt.Sprintf("bad return values: %v, %v", success, err))
				return err
			}
			if success == false {
				panic(fmt.Sprintf("bad return values: %v, %v", success, err))
			}

			greeting, err := serverClient.PingWithReply(ctx, "Oliver")
			if err != nil {
				panic(err)
			}
			if greeting != "hello Oliver" {
				panic("incorrect greeting")
			}
			greeting2, err := echoClient.Echo(ctx, "weee", []string{"Hello", ", ", "World", "!"}, map[string]int64{
				"five": 5,
				"two": 2,
				"thirtysix": 36,
			})
			if err != nil {
				panic(err)
			}
			if greeting2 != "weee" {
				panic("incorrect greeting 2")
			}

			_, err = serverClient.TestMethod(ctx, "hello", true, 1234, 654, 3.454, 3.14)
			if err != nil {
				panic(err)
			}

			return nil
		},
		DisconnectHook: func(c *RPCConnection, err RPCError) {
			fmt.Printf("Client: disconnected from server\n")
		},
	}).Wait()
}

type server struct {
	client        *RPCPingpongClient
	authenticated bool
}

func (s *server) Authenticate(ctx context.Context, username string, password string) (success bool, err error) {
	fmt.Printf("WEEEE\n")
	if username == "batman" && password == "robin" {
		s.authenticated = true
		return true, nil
	}
	return false, nil
}

func (s *server) PingWithReply(ctx context.Context, name string) (greeting string, err error) {
	//s.client.PingWithReply(name, err)
	return "hello " + name, nil
}

func (s *server) TestMethod(ctx context.Context, a string, b bool, c int64, d int64, e float32, f float64) (success bool, err error) {
	return a == "hello" && b && c == 1234 && d == 654 && e == 3.454 && f == 3.14, nil
}

type echoServer struct {
	client        *RPCEchoClient
	authenticated bool
}

func (s *echoServer) Echo(ctx context.Context, input string, names []string, values map[string]int64) (string, error) {
	if values == nil || values["five"] != 5 || values["two"] != 2 ||
		values["thirtysix"] != 36 || len(names) != 4 {
		return "", fmt.Errorf("NOPE.")
	}
	return input, nil
}


func (s *echoServer) Ping(ctx context.Context) (string, error) {
	return "", nil
}
