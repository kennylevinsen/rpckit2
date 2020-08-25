package main

import (
	"context"
)

// A struct
type Ssss struct {

	// WAAAAH
	Wee string `json:"wee"`
}

// The PingpongProtocol interface defines the pingpong protocol.
type PingpongProtocol interface {
	// The simplest of tests
	SimpleTest(ctx context.Context, reqVinteger int64, reqVint64 int64, reqVfloat float32, reqVdouble float64, reqVbool bool, reqVstring string, reqVbytes []byte, reqVstruct *Ssss) (respVinteger int64, respVint64 int64, respVfloat float32, respVdouble float64, respVbool bool, respVstring string, respVbytes []byte, respVstruct *Ssss, err error)

	// The simplest of tests, but with arrays
	ArrayTest(ctx context.Context, reqVinteger []int64, reqVint64 []int64, reqVfloat []float32, reqVdouble []float64, reqVbool []bool, reqVstring []string, reqVbytes [][]byte, reqVstruct []*Ssss) (respVinteger []int64, respVint64 []int64, respVfloat []float32, respVdouble []float64, respVbool []bool, respVstring []string, respVbytes [][]byte, respVstruct []*Ssss, err error)
}
