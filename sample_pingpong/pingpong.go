package main

import (
	"errors"
	"fmt"
	"net"
	"time"
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

			connection, err := NewRPCConnectionWithTimeout(conn, 2 * time.Second)
			if err != nil {
				panic(err)
			}
			RegisterRPCCallHandler(connection, &server{})
		}
	}()

	// dial it
	conn, err := net.Dial("tcp", "127.0.0.1:9999")
	if err != nil {
		panic(err)
	}
	connection, err := NewRPCConnectionWithTimeout(conn, 2 * time.Second)
	if err != nil {
		panic(err)
	}
	serverClient := NewServerClient(connection)
	success, err := serverClient.Authenticate("batman", "robin")
	if err != nil || success == false {
		panic(fmt.Sprintf("bad return values: %v, %v", success, err))
	}
	greeting, err := serverClient.PingWithReply("Oliver")
	if err != nil {
		panic(err)
	}

	_, err = serverClient.TestMethod("hello", true, 1234, 654, 3.454, 3.14)
	if err != nil {
		panic(err)
	}
	fmt.Println("greeting:", greeting)
	fmt.Println("done")
	time.Sleep(time.Millisecond * 200)
}

type server struct {
	client        *ServerClient
	authenticated bool
}

func (s *server) AllowCall(m ServerMethod) (err error) {
	if s.authenticated {
		return nil
	}
	if m == ServerMethodPingWithReply {
		return nil
	}
	return errors.New("not authenticated")
}

func (s *server) Authenticate(username string, password string) (success bool, err error) {
	fmt.Printf("AUTHENTICATE: %s, %s\n", username, password)
	if username == "batman" && password == "robin" {
		s.authenticated = true
		fmt.Printf("OK\n")
		return true, nil
	}
	fmt.Printf("NOT OK\n ")
	return false, nil
}

func (s *server) PingWithReply(name string) (greeting string, err error) {
	//s.client.PingWithReply(name, err)
	return "hello " + name, nil
}

func (s *server) TestMethod(a string, b bool, c int64, d int64, e float32, f float64) (success bool, err error) {
	fmt.Printf("a: %s, b: %t, c: %d, d: %d, e: %f, f: %f\n", a, b, c, d, e, f)
	return true, nil
}
