package main

import (
	"context"
)

// Channel info object to contain details of channel
type ChannelInfo struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

// The PingpongProtocol interface defines the pingpong protocol.
type PingpongProtocol interface {
	// The simplest of tests
	SimpleTest(ctx context.Context, reqVinteger int64, reqVint64 int64, reqVfloat float32, reqVdouble float64, reqVbool bool, reqVstring string, reqVbytes []byte, reqChannels ChannelInfo) (respVinteger int64, respVint64 int64, respVfloat float32, respVdouble float64, respVbool bool, respVstring string, respVbytes []byte, respChannels ChannelInfo, err error)

	// The simplest of tests, but with arrays
	ArrayTest(ctx context.Context, reqVinteger []int64, reqVint64 []int64, reqVfloat []float32, reqVdouble []float64, reqVbool []bool, reqVstring []string, reqVbytes [][]byte, reqChannels []ChannelInfo) (respVinteger []int64, respVint64 []int64, respVfloat []float32, respVdouble []float64, respVbool []bool, respVstring []string, respVbytes [][]byte, respChannels []ChannelInfo, err error)
}

// The BackendProtocol interface defines the Backend protocol.
type BackendProtocol interface {
	// Authenticate using either (token or e-mail).
	Authenticate(ctx context.Context, reqToken string, reqEmail string) (respToken string, err error)

	// List my active channels
	ListChannels(ctx context.Context) (respGreeting string, respChannels []ChannelInfo, err error)

	// Set the avatar for the currently authenticated user
	SetAvatar(ctx context.Context, reqImage []byte) (err error)

	// Set the name of the current user
	SetName(ctx context.Context, reqName string) (err error)

	// Register for push notifications
	RegisterNotifications(ctx context.Context, reqName string, reqDevicetype string) (err error)
}
