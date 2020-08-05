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

		c := NewRPCConnection(&RPCOptions{
			Conn: conn,
			Servers: []RPCServer{
				RPCPingpongServer(&server{}),
			},
		})
		c.Ready()
		fmt.Printf("Stuff\n")
		client := NewRPCPingpongClient(c)
		// channels := []ChannelInfo{
		// 	ChannelInfo{ID: 1, Title: "werwer"},
		// 	ChannelInfo{ID: 2, Title: "werwjkljlkjar"},
		// }
		integer, int64, float, double, bool, string, bytes, channel, err := client.SimpleTest(context.Background(), 1, 2, 3.1, 4.1, true, "yelp", []byte("WOOOO"), ChannelInfo{ID: 1, Title: "werwer"},)
		fmt.Printf("Got: %d, %d, %f, %f, %t, %s, %v, %v, %v\n", integer, int64, float, double, bool, string, bytes, channel, err)
		// ainteger, aint64, afloat, adouble, abool, astring, abytes, achannels, err := client.ArrayTest(context.Background(), 1, 2, 3.1, 4.1, true, "yelp", []byte("WOOOO"), channels)
		// fmt.Printf("Got: %d, %d, %f, %f, %t, %s, %v, %v, %v\n", ainteger, aint64, afloat, adouble, abool, astring, abytes, achannels, err)
	}
}

type server struct {}

func (c *server) SimpleTest(ctx context.Context, integer int64, int64 int64, float float32, double float64, bool bool, string string, bytes []byte, channel ChannelInfo) (int64, int64, float32, float64, bool, string, []byte, ChannelInfo, error) {
	fmt.Printf("Got: %d, %d, %f, %f, %t, %s, %v, %v\n", integer, int64, float, double, bool, string, bytes, channel)
	return integer, int64, float, double, bool, string, bytes, channel, nil
}

func (c *server) ArrayTest(ctx context.Context, integer []int64, int64 []int64, float []float32, double []float64, bool []bool, string []string, bytes [][]byte, v []ChannelInfo) ([]int64, []int64, []float32, []float64, []bool, []string, [][]byte, []ChannelInfo, error) {

	fmt.Printf("Got: %v, %v, %v, %v, %v, %v, %v, %v\n", integer, int64, float, double, bool, string, bytes, v)
	return integer, int64, float, double, bool, string, bytes, v, nil
}
