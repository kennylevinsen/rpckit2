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

			connection, err := NewRPCConnection(conn)
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
	connection, err := NewRPCConnection(conn)
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
