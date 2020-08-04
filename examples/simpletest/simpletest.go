package main

import (
	"context"
	"time"
	"net"
	"fmt"
)

var now = time.Now()

func main() {
	// listen for network connections
	l1, err := net.Listen("tcp", "127.0.0.1:12345")
	if err != nil {
		panic(err)
	}
	for {
		conn, err := l1.Accept()
		if err != nil {
			panic(err)
		}

		NewRPCConnection(&RPCOptions{
			Conn: conn,
			Servers: []RPCServer{
				RPCPingpongServer(&server{}),
			},
			ConnectHook: func(ctx context.Context, c *RPCConnection) error {
				fmt.Printf("Stuff\n")
				client := NewRPCPingpongClient(c)
				integer, int64, float, double, bool, string, bytes, err := client.SimpleTest(ctx, 1, 2, 3.1, 4.1, true, "yelp", []byte("WOOOO"))
				fmt.Printf("Got: %d, %d, %f, %f, %t, %s, %v, %v\n", integer, int64, float, double, bool, string, bytes, err)
				return nil
			},
		})
	}
}

type server struct {}

func (c *server) SimpleTest(ctx context.Context, integer int64, int64 int64, float float32, double float64, bool bool, string string, bytes []byte) (int64, int64, float32, float64, bool, string, []byte, error) {
	fmt.Printf("Got: %d, %d, %f, %f, %t, %s, %v\n", integer, int64, float, double, bool, string, bytes)
	return integer, int64, float, double, bool, string, bytes, nil
}

func (c *server) ArrayTest(ctx context.Context, integer []int64, int64 []int64, float []float32, double []float64, bool []bool, string []string, bytes [][]byte) ([]int64, []int64, []float32, []float64, []bool, []string, [][]byte, error) {

	fmt.Printf("Got: %v, %v, %v, %v, %v, %v, %v\n", integer, int64, float, double, bool, string, bytes)
	return integer, int64, float, double, bool, string, bytes, nil
}
