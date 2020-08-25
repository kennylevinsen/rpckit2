package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/satori/go.uuid"
)

var now = time.Now()

func testRPC() {
	// listen for network connections
	l1, err := net.Listen("tcp", ":9999")
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
				Servers: []RPCServer{
					RPCPingpongServer(&server{}),
					RPCEchoServer(&echoServer{}),
				},
			})
		}
	}()

	NewRPCConnection(&RPCOptions{
		Dialer: func() (net.Conn, error) {
			return net.Dial("tcp", "127.0.0.1:9999")
		},
		ConnectHook: func(ctx context.Context, c *RPCConnection) error {
			serverClient := NewRPCPingpongClient(c)
			echoClient := NewRPCEchoClient(c)

			success, err := serverClient.Authenticate(ctx, "batman", "robin")
			if err != nil {
				panic(fmt.Sprintf("bad return values: %v, %v", success, err))
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
			greeting2, _, err := echoClient.Echo(ctx,
				"weee",
				[]string{"Hello", ", ", "World", "!"},
				map[string]map[string]int64{
					"five":      map[string]int64{"five": 5, "fiftyfive": 55, "fivehundredandfiftyfive": 555},
					"two":       map[string]int64{"two": 2, "twentytwo": 22, "twohundredandtwentytwo": 222},
					"thirtysix": map[string]int64{"thirtysix": 36, "threethousandandthirtysix": 3636, "threehundredsixtythreethousandthirtysix": 363636},
				},
				map[string]int64{"cpu": 12345},
				&EchoThing{
					Wee:         "asdf",
					Stuff:       map[string]int64{"cpu": 1234},
					Anothertime: now,
				},
				now,
				uuid.UUID{},
			)
			if err != nil {
				panic(err)
			}
			if greeting2 != "weee" {
				panic("incorrect greeting 2")
			}

			_, err = serverClient.TestMethod(ctx, "hello", true, 1234, 654, 3.454, 3.14, now)
			if err != nil {
				panic(err)
			}

			fmt.Printf("RPC test complete\n")
			c.Close()
			return nil
		},
	}).Wait()
}

func testHTTP() {
	l2, err := net.Listen("tcp", ":9998")
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	go func() {
		mux := http.NewServeMux()

		HTTPEchoServer(&echoServer{}).RegisterToMux(mux)
		HTTPPingpongServer(&server{}).RegisterToMux(mux)

		http.Serve(l2, mux)
	}()

	hc, err := NewHTTPClient("http://localhost:9998")
	if err != nil {
		panic(err)
	}

	serverClient := NewHTTPPingpongClient(hc)
	echoClient := NewHTTPEchoClient(hc)

	success, err := serverClient.Authenticate(ctx, "batman", "robin")
	if err != nil {
		panic(fmt.Sprintf("bad return values: %v, %v", success, err))
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
	greeting2, _, err := echoClient.Echo(ctx, "weee", []string{"Hello", ", ", "World", "!"}, map[string]map[string]int64{
		"five":      map[string]int64{"five": 5, "fiftyfive": 55, "fivehundredandfiftyfive": 555},
		"two":       map[string]int64{"two": 2, "twentytwo": 22, "twohundredandtwentytwo": 222},
		"thirtysix": map[string]int64{"thirtysix": 36, "threethousandandthirtysix": 3636, "threehundredsixtythreethousandthirtysix": 363636},
	}, map[string]int64{"cpu": 12345}, &EchoThing{Wee: "asdf", Stuff: map[string]int64{"cpu": 1234}}, now, uuid.UUID{})
	if err != nil {
		panic(err)
	}
	if greeting2 != "weee" {
		panic("incorrect greeting 2")
	}

	_, err = serverClient.TestMethod(ctx, "hello", true, 1234, 654, 3.454, 3.14, now)
	if err != nil {
		panic(err)
	}

	b, err := echoClient.ByteTest(ctx, []byte("Hello there!"))
	if err != nil {
		panic(err)
	}
	if string(b) != "Hello there!" {
		panic(fmt.Sprintf("string was ", b))
	}

	fmt.Printf("HTTP test complete\n")
}

func main() {
	testHTTP()
	testRPC()
}

type server struct {
	client        *RPCPingpongClient
	authenticated bool
}

func (s *server) Authenticate(ctx context.Context, username string, password string) (success bool, err error) {
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

func (s *server) TestMethod(ctx context.Context, a string, b bool, c int64, d int64, e float32, f float64, t time.Time) (success bool, err error) {
	return a == "hello" && b && c == 1234 && d == 654 && e == 3.454 && f == 3.14 && t == now, nil
}

type echoServer struct {
	client        *RPCEchoClient
	authenticated bool
}

func (s *echoServer) Echo(ctx context.Context, input string, names []string, values map[string]map[string]int64, values2 map[string]int64, echoThing *EchoThing, atime time.Time, id uuid.UUID) (string, time.Time, error) {
	fmt.Printf("VALUES: %#v, %#v, %#v, %v, %v\n", values, values2, echoThing, atime, id)
	return input, time.Time{}, nil
}

func (s *echoServer) Ping(ctx context.Context) (string, error) {
	return "", nil
}

func (s *echoServer) ByteTest(ctx context.Context, input []byte) ([]byte, error) {
	return input, nil
}
