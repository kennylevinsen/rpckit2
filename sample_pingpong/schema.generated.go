package main

import "context"

// The PingpongMethods type defines a server that can
// process requests from the pingpong protocol.
type PingpongMethods interface {
	// Connect
	Authenticate(
		ctx context.Context,
		reqUsername string,
		reqPassword string,
	) (
		respSuccess bool,
		err error,
	)
	// Echo
	PingWithReply(
		ctx context.Context,
		reqName string,
	) (
		respGreeting string,
		err error,
	)
	// Echo
	TestMethod(
		ctx context.Context,
		reqString string,
		reqBool bool,
		reqInt64 int64,
		reqInt int64,
		reqFloat float32,
		reqDouble float64,
	) (
		respSuccess bool,
		err error,
	)
}

// The EchoMethods type defines a server that can
// process requests from the echo protocol.
type EchoMethods interface {
	// Echo
	Echo(
		ctx context.Context,
		reqInput string,
		reqNames []string,
		reqValues map[string]int64,
	) (
		respOutput string,
		err error,
	)
	// Ping
	Ping(
		ctx context.Context,
	) (
		respOutput string,
		err error,
	)
}
