package main

import (
	"fmt"
	"net"
	"time"
	"context"
)

func main() {
	// listen for network connections
	l, err := net.Listen("tcp", ":9999")
	if err != nil {
		panic(err)
	}
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				panic(err)
			}

			NewRPCBuilder().
				Conn(conn).
				Server(
					PingpongHandler(&server{}),
					EchoHandler(&echoServer{}),
				).
				ConnectHook(func(c *RPCConnection) error {
					fmt.Printf("Server: user connected\n")
					time.Sleep(5 * time.Second)
					conn.Close()
					return nil
				}).
				DisconnectHook(func(c *RPCConnection, err RPCError) {
					fmt.Printf("Server: user disconnected\n")
				}).
				Build()
		}
	}()

	ctx := context.Background()

	ready := make(chan struct{}, 0)

	c := NewRPCBuilder().
		Dialer(func() (net.Conn, error) {
			return net.Dial("tcp", "127.0.0.1:9999")
		}).
		ConnectHook(func(c *RPCConnection) error {
			fmt.Printf("Client: connected to server\n")
			success, err := NewRPCPingpongClient(c).Authenticate(ctx, "batman", "robin")
			if err != nil {
				panic(fmt.Sprintf("bad return values: %v, %v", success, err))
				return err
			}
			if success == false {
				panic(fmt.Sprintf("bad return values: %v, %v", success, err))
			}

			select {
			case <-ready:
			default:
				close(ready)
			}
			return nil
		}).
		DisconnectHook(func(c *RPCConnection, err RPCError) {
			fmt.Printf("Client: disconnected from server\n")
		}).
		Build()

	<-ready

	serverClient := NewRPCPingpongClient(c)
	echoClient := NewRPCEchoClient(c)

	greeting, err := serverClient.PingWithReply(ctx, "Oliver")
	if err != nil {
		panic(err)
	}
	// time.Sleep(10 * time.Second)

	greeting2, err := echoClient.Echo(ctx, "weee", []string{"Hello", ", ", "World", "!"}, map[string]int64{
		"five": 5,
		"two": 2,
		"thirtysix": 36,
	})
	if err != nil {
		panic(err)
	}

	// time.Sleep(10 * time.Second)

	_, err = serverClient.TestMethod(ctx, "hello", true, 1234, 654, 3.454, 3.14)
	if err != nil {
		panic(err)
	}
	fmt.Println("greeting:", greeting)
	fmt.Println("greeting2:", greeting2)
	fmt.Println("done")

	c.Close()

	time.Sleep(2 * time.Second)
}

type server struct {
	client        *RPCPingpongClient
	authenticated bool
}

func (s *server) Authenticate(ctx context.Context, username string, password string) (success bool, err error) {
	fmt.Printf("AUTHENTICATE: %s, %s\n", username, password)
	if username == "batman" && password == "robin" {
		s.authenticated = true
		fmt.Printf("OK\n")
		return true, nil
	}
	fmt.Printf("NOT OK\n ")
	return false, nil
}

func (s *server) PingWithReply(ctx context.Context, name string) (greeting string, err error) {
	//s.client.PingWithReply(name, err)
	return "hello " + name, nil
}

func (s *server) TestMethod(ctx context.Context, a string, b bool, c int64, d int64, e float32, f float64) (success bool, err error) {
	fmt.Printf("a: %s, b: %t, c: %d, d: %d, e: %f, f: %f\n", a, b, c, d, e, f)
	return true, nil
}

type echoServer struct {
	client        *RPCEchoClient
	authenticated bool
}

func (s *echoServer) Echo(ctx context.Context, input string, names []string, values map[string]int64) (string, error) {
	fmt.Printf("ECHO: %#v, %#v\n", names, values)
	return input, nil
}


func (s *echoServer) Ping(ctx context.Context) (string, error) {
	return "", nil
}
