package main

import (
	"errors"
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

			connection, err := NewRPCConnection(conn, RPCTimeout(5 * time.Minute))
			if err != nil {
				panic(err)
			}
			RegisterPingpongHandler(connection, &server{})
			RegisterEchoHandler(connection, &echoServer{})

			connection.Start()
		}
	}()

	// dial it
	conn, err := net.Dial("tcp", "127.0.0.1:9999")
	if err != nil {
		panic(err)
	}
	connection, err := NewRPCConnection(conn,
		RPCTimeout(5 * time.Minute),
		RPCKeepAlive(5 * time.Second),
		RPCDynamicKeepAlive(true))
	if err != nil {
		panic(err)
	}
	serverClient := NewPingpongClient(connection)
	echoClient := NewEchoClient(connection)

	connection.Start()

	ctx := context.Background()

	success, err := serverClient.Authenticate(ctx, "batman", "robin")
	if err != nil || success == false {
		panic(fmt.Sprintf("bad return values: %v, %v", success, err))
	}
	greeting, err := serverClient.PingWithReply(ctx, "Oliver")
	if err != nil {
		panic(err)
	}

	greeting2, err := echoClient.Echo(ctx, "weee", []string{"Hello", ", ", "World", "!"}, map[string]int64{
		"five": 5,
		"two": 2,
		"thirtysix": 36,
	})
	if err != nil {
		panic(err)
	}

	_, err = serverClient.TestMethod(ctx, "hello", true, 1234, 654, 3.454, 3.14)
	if err != nil {
		panic(err)
	}
	fmt.Println("greeting:", greeting)
	fmt.Println("greeting2:", greeting2)
	fmt.Println("done")
	time.Sleep(time.Millisecond * 200)
}

type server struct {
	client        *PingpongClient
	authenticated bool
}

func (s *server) AllowCall(m PingpongMethod) (err error) {
	if s.authenticated {
		return nil
	}
	if m == PingpongMethod_PingWithReply {
		return nil
	}
	return errors.New("not authenticated")
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
	client        *EchoClient
	authenticated bool
}

func (s *echoServer) Echo(ctx context.Context, input string, names []string, values map[string]int64) (string, error) {
	fmt.Printf("ECHO: %#v, %#v\n", names, values)
	return input, nil
}
