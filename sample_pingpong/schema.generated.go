package main

import "context"

// The PingpongProtocol interface defines the pingpong protocol.
type PingpongProtocol interface {
	// Authenticate using username and password
	Authenticate(ctx context.Context, reqUsername string, reqPassword string) (respSuccess bool, err error)
	// PingWithReply replies with a greeting based on the provided name
	PingWithReply(ctx context.Context, reqName string) (respGreeting string, err error)
	// TestMethod is a simple type test
	TestMethod(ctx context.Context, reqString string, reqBool bool, reqInt64 int64, reqInt int64, reqFloat float32, reqDouble float64) (respSuccess bool, err error)
}

// The EchoProtocol interface defines the echo protocol.
type EchoProtocol interface {
	// Echo is yet another type test
	Echo(ctx context.Context, reqInput string, reqNames []string, reqValues map[string]int64) (respOutput string, err error)
	// Ping is a simple no-input test
	Ping(ctx context.Context) (respOutput string, err error)
}
