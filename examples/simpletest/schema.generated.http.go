package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"

	"golang.org/x/net/publicsuffix"
)

// HTTPClient wraps a http client configured for use with protocol wrappers.
type HTTPClient struct {
	client  *http.Client
	baseURL string
}

// NewHTTPClient creates a new HTTPClient.
func NewHTTPClient(baseURL string) (*HTTPClient, error) {
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, err
	}
	return &HTTPClient{
		client: &http.Client{
			Transport: http.DefaultClient.Transport,
			Jar:       jar,
		},
		baseURL: baseURL,
	}, nil
}

func (c *HTTPClient) newRequest(method, url string, body io.Reader) (*http.Request, error) {
	return http.NewRequest(method, c.baseURL+"/"+url, body)
}

func (c *HTTPClient) do(req *http.Request) (*http.Response, error) {
	return c.client.Do(req)
}

type HTTPServer interface {
	RegisterToMux(*http.ServeMux)
}

// The HTTPPingpongClient type is a HTTP client for the pingpong protocol.
type HTTPPingpongClient struct {
	client  *HTTPClient
	baseURL string
}

// NewHTTPPingpongClient(c *HTTPClient) creates a new HTTP client for the pingpong protocol.
func NewHTTPPingpongClient(c *HTTPClient) *HTTPPingpongClient {
	return &HTTPPingpongClient{
		client:  c,
		baseURL: "pingpong",
	}
}

func (c *HTTPPingpongClient) newRequest(method, url string, body io.Reader) (*http.Request, error) {
	return c.client.newRequest(method, c.baseURL+"/"+url, body)
}

type httpReqProtoPingpongMethodSimpleTest struct {
	Vinteger int64   `json:"vinteger"`
	Vint64   int64   `json:"vint64"`
	Vfloat   float32 `json:"vfloat"`
	Vdouble  float64 `json:"vdouble"`
	Vbool    bool    `json:"vbool"`
	Vstring  string  `json:"vstring"`
	Vbytes   []byte  `json:"vbytes"`
}

type httpRespProtoPingpongMethodSimpleTest struct {
	Vinteger int64   `json:"vinteger"`
	Vint64   int64   `json:"vint64"`
	Vfloat   float32 `json:"vfloat"`
	Vdouble  float64 `json:"vdouble"`
	Vbool    bool    `json:"vbool"`
	Vstring  string  `json:"vstring"`
	Vbytes   []byte  `json:"vbytes"`
}

// The simplest of tests
func (c *HTTPPingpongClient) SimpleTest(ctx context.Context, reqVinteger int64, reqVint64 int64, reqVfloat float32, reqVdouble float64, reqVbool bool, reqVstring string, reqVbytes []byte) (respVinteger int64, respVint64 int64, respVfloat float32, respVdouble float64, respVbool bool, respVstring string, respVbytes []byte, err error) {
	var (
		b        []byte
		req      *http.Request
		resp     *http.Response
		reqbody  httpReqProtoPingpongMethodSimpleTest
		respbody httpRespProtoPingpongMethodSimpleTest
	)

	reqbody = httpReqProtoPingpongMethodSimpleTest{
		Vinteger: reqVinteger,
		Vint64:   reqVint64,
		Vfloat:   reqVfloat,
		Vdouble:  reqVdouble,
		Vbool:    reqVbool,
		Vstring:  reqVstring,
		Vbytes:   reqVbytes,
	}

	if b, err = json.Marshal(&reqbody); err != nil {
		return
	}

	if req, err = c.newRequest("POST", "simpletest", bytes.NewReader(b)); err != nil {
		return
	}

	req = req.WithContext(ctx)
	if resp, err = c.client.do(req); err != nil {
		return
	}

	b, err = ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return
	}

	if !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
		var errorbody struct {
			Error string `json:"error"`
		}

		if err = json.Unmarshal(b, &errorbody); err != nil {
			return
		}

		err = errors.New(errorbody.Error)
		return
	}

	if err = json.Unmarshal(b, &respbody); err != nil {
		return
	}
	respVinteger = respbody.Vinteger
	respVint64 = respbody.Vint64
	respVfloat = respbody.Vfloat
	respVdouble = respbody.Vdouble
	respVbool = respbody.Vbool
	respVstring = respbody.Vstring
	respVbytes = respbody.Vbytes

	return
}

type httpReqProtoPingpongMethodArrayTest struct {
	Vinteger []int64   `json:"vinteger"`
	Vint64   []int64   `json:"vint64"`
	Vfloat   []float32 `json:"vfloat"`
	Vdouble  []float64 `json:"vdouble"`
	Vbool    []bool    `json:"vbool"`
	Vstring  []string  `json:"vstring"`
	Vbytes   [][]byte  `json:"vbytes"`
}

type httpRespProtoPingpongMethodArrayTest struct {
	Vinteger []int64   `json:"vinteger"`
	Vint64   []int64   `json:"vint64"`
	Vfloat   []float32 `json:"vfloat"`
	Vdouble  []float64 `json:"vdouble"`
	Vbool    []bool    `json:"vbool"`
	Vstring  []string  `json:"vstring"`
	Vbytes   [][]byte  `json:"vbytes"`
}

// The simplest of tests, but with arrays
func (c *HTTPPingpongClient) ArrayTest(ctx context.Context, reqVinteger []int64, reqVint64 []int64, reqVfloat []float32, reqVdouble []float64, reqVbool []bool, reqVstring []string, reqVbytes [][]byte) (respVinteger []int64, respVint64 []int64, respVfloat []float32, respVdouble []float64, respVbool []bool, respVstring []string, respVbytes [][]byte, err error) {
	var (
		b        []byte
		req      *http.Request
		resp     *http.Response
		reqbody  httpReqProtoPingpongMethodArrayTest
		respbody httpRespProtoPingpongMethodArrayTest
	)

	reqbody = httpReqProtoPingpongMethodArrayTest{
		Vinteger: reqVinteger,
		Vint64:   reqVint64,
		Vfloat:   reqVfloat,
		Vdouble:  reqVdouble,
		Vbool:    reqVbool,
		Vstring:  reqVstring,
		Vbytes:   reqVbytes,
	}

	if b, err = json.Marshal(&reqbody); err != nil {
		return
	}

	if req, err = c.newRequest("POST", "arraytest", bytes.NewReader(b)); err != nil {
		return
	}

	req = req.WithContext(ctx)
	if resp, err = c.client.do(req); err != nil {
		return
	}

	b, err = ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return
	}

	if !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
		var errorbody struct {
			Error string `json:"error"`
		}

		if err = json.Unmarshal(b, &errorbody); err != nil {
			return
		}

		err = errors.New(errorbody.Error)
		return
	}

	if err = json.Unmarshal(b, &respbody); err != nil {
		return
	}
	respVinteger = respbody.Vinteger
	respVint64 = respbody.Vint64
	respVfloat = respbody.Vfloat
	respVdouble = respbody.Vdouble
	respVbool = respbody.Vbool
	respVstring = respbody.Vstring
	respVbytes = respbody.Vbytes

	return
}

// HTTPPingpongServer creates a new HTTPServer for the pingpong protocol.
func HTTPPingpongServer(methods PingpongProtocol) HTTPServer {
	return &httpCallServerForPingpong{methods: methods}
}

type httpCallServerForPingpong struct {
	methods PingpongProtocol
}

func (c *httpCallServerForPingpong) RegisterToMux(m *http.ServeMux) {
	m.HandleFunc("/pingpong/simpletest", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var (
			err      error
			b        []byte
			reqbody  httpReqProtoPingpongMethodSimpleTest
			respbody httpRespProtoPingpongMethodSimpleTest
		)

		b, err = ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if err = json.Unmarshal(b, &reqbody); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		respbody.Vinteger, respbody.Vint64, respbody.Vfloat, respbody.Vdouble, respbody.Vbool, respbody.Vstring, respbody.Vbytes, err = c.methods.SimpleTest(r.Context(), reqbody.Vinteger, reqbody.Vint64, reqbody.Vfloat, reqbody.Vdouble, reqbody.Vbool, reqbody.Vstring, reqbody.Vbytes)
		if err != nil {
			if header, ok := err.(interface{ StatusCode() int }); ok {
				w.WriteHeader(header.StatusCode())
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}

			errorbody := struct {
				Error interface{} `json:"error"`
			}{
				Error: err,
			}
			if b, err = json.Marshal(&errorbody); err != nil {
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write(b)
			return
		}

		if b, err = json.Marshal(&respbody); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			// TODO: Add error!
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(b)
	})
	m.HandleFunc("/pingpong/arraytest", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var (
			err      error
			b        []byte
			reqbody  httpReqProtoPingpongMethodArrayTest
			respbody httpRespProtoPingpongMethodArrayTest
		)

		b, err = ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if err = json.Unmarshal(b, &reqbody); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		respbody.Vinteger, respbody.Vint64, respbody.Vfloat, respbody.Vdouble, respbody.Vbool, respbody.Vstring, respbody.Vbytes, err = c.methods.ArrayTest(r.Context(), reqbody.Vinteger, reqbody.Vint64, reqbody.Vfloat, reqbody.Vdouble, reqbody.Vbool, reqbody.Vstring, reqbody.Vbytes)
		if err != nil {
			if header, ok := err.(interface{ StatusCode() int }); ok {
				w.WriteHeader(header.StatusCode())
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}

			errorbody := struct {
				Error interface{} `json:"error"`
			}{
				Error: err,
			}
			if b, err = json.Marshal(&errorbody); err != nil {
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write(b)
			return
		}

		if b, err = json.Marshal(&respbody); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			// TODO: Add error!
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(b)
	})
}
