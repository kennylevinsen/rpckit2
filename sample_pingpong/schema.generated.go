package main

import (
	"context"
	"time"
)

// Echo thing
type EchoThing struct {

	// WAAAAH
	Wee string `json:"wee"`

	// Woo describes the woo factor
	Woo string `json:"woo"`

	// Weee
	Stuff map[string]int64 `json:"stuff"`

	// My wao time
	Anothertime time.Time `json:"anothertime,omitempty"`

	// Datetime mapsimap
	Mydatetimemap map[time.Time]time.Time `json:"mydatetimemap"`
}

func (s *EchoThing) GetAnothertime() time.Time { return s.Anothertime }
func (s *EchoThing) SetAnothertime(v time.Time) {
	s.Anothertime = v
}

// The PingpongProtocol interface defines the pingpong protocol.
type PingpongProtocol interface {
	// Authenticate using username and password
	Authenticate(ctx context.Context, reqUsername string, reqPassword string) (respSuccess bool, err error)

	// PingWithReply replies with a greeting based on the provided name
	PingWithReply(ctx context.Context, reqName string) (respGreeting string, err error)

	// TestMethod is a simple type test
	TestMethod(ctx context.Context, reqString string, reqBool bool, reqInt64 int64, reqInt int64, reqFloat float32, reqDouble float64, reqDatetime time.Time) (respSuccess bool, err error)
}

// The EchoProtocol interface defines the echo protocol.
type EchoProtocol interface {
	// Echo is yet another type test
	Echo(ctx context.Context, reqInput string, reqNames []string, reqValues map[string]map[string]int64, reqValues2 map[string]int64, reqSomething EchoThing, reqMytime time.Time) (respOutput string, err error)

	// Ping is a simple no-input test
	Ping(ctx context.Context) (respOutput string, err error)

	// ByteTest is a byte test
	ByteTest(ctx context.Context, reqInput []byte) (respOutput []byte, err error)
}
