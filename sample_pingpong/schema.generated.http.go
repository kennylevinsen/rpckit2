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

	"time"

	"github.com/satori/go.uuid"
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

type httpReqProtoPingpongMethodAuthenticate struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type httpRespProtoPingpongMethodAuthenticate struct {
	Success bool `json:"success"`
}

// The HTTPAuthenticateClient type is a HTTP client for the Authenticate protocol.
type HTTPAuthenticateClient struct {
	client  *HTTPClient
	baseURL string
}

// NewHTTPAuthenticateClient(c *HTTPClient) creates a new HTTP client for the Authenticate protocol.
func NewHTTPAuthenticateClient(c *HTTPClient) *HTTPAuthenticateClient {
	return &HTTPAuthenticateClient{
		client:  c,
		baseURL: "Authenticate",
	}
}

func (c *HTTPAuthenticateClient) newRequest(method, url string, body io.Reader) (*http.Request, error) {
	return c.client.newRequest(method, c.baseURL+"/"+url, body)
}

type httpReqProtoPingpongMethodPingWithReply struct {
	Name string `json:"name"`
}

type httpRespProtoPingpongMethodPingWithReply struct {
	Greeting string `json:"greeting"`
}

// The HTTPPingWithReplyClient type is a HTTP client for the PingWithReply protocol.
type HTTPPingWithReplyClient struct {
	client  *HTTPClient
	baseURL string
}

// NewHTTPPingWithReplyClient(c *HTTPClient) creates a new HTTP client for the PingWithReply protocol.
func NewHTTPPingWithReplyClient(c *HTTPClient) *HTTPPingWithReplyClient {
	return &HTTPPingWithReplyClient{
		client:  c,
		baseURL: "PingWithReply",
	}
}

func (c *HTTPPingWithReplyClient) newRequest(method, url string, body io.Reader) (*http.Request, error) {
	return c.client.newRequest(method, c.baseURL+"/"+url, body)
}

type httpReqProtoPingpongMethodTestMethod struct {
	String   string    `json:"string"`
	Bool     bool      `json:"bool"`
	Int64    int64     `json:"int64"`
	Int      int64     `json:"int"`
	Float    float32   `json:"float"`
	Double   float64   `json:"double"`
	Datetime time.Time `json:"datetime"`
}

type httpRespProtoPingpongMethodTestMethod struct {
	Success bool `json:"success"`
}

// The HTTPTestMethodClient type is a HTTP client for the TestMethod protocol.
type HTTPTestMethodClient struct {
	client  *HTTPClient
	baseURL string
}

// NewHTTPTestMethodClient(c *HTTPClient) creates a new HTTP client for the TestMethod protocol.
func NewHTTPTestMethodClient(c *HTTPClient) *HTTPTestMethodClient {
	return &HTTPTestMethodClient{
		client:  c,
		baseURL: "TestMethod",
	}
}

func (c *HTTPTestMethodClient) newRequest(method, url string, body io.Reader) (*http.Request, error) {
	return c.client.newRequest(method, c.baseURL+"/"+url, body)
}

// HTTPPingpongServer creates a new HTTPServer for the pingpong protocol.
func HTTPPingpongServer(methods PingpongProtocol) HTTPServer {
	return &httpCallServerForPingpong{methods: methods}
}

type httpCallServerForPingpong struct {
	methods PingpongProtocol
}

func (c *httpCallServerForPingpong) RegisterToMux(m *http.ServeMux) {
	m.HandleFunc("/pingpong/authenticate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var (
			err      error
			b        []byte
			reqbody  httpReqProtoPingpongMethodAuthenticate
			respbody httpRespProtoPingpongMethodAuthenticate
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

		respbody.Success, err = c.methods.Authenticate(r.Context(), reqbody.Username, reqbody.Password)
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
	m.HandleFunc("/pingpong/pingwithreply", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var (
			err      error
			b        []byte
			reqbody  httpReqProtoPingpongMethodPingWithReply
			respbody httpRespProtoPingpongMethodPingWithReply
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

		respbody.Greeting, err = c.methods.PingWithReply(r.Context(), reqbody.Name)
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
	m.HandleFunc("/pingpong/testmethod", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var (
			err      error
			b        []byte
			reqbody  httpReqProtoPingpongMethodTestMethod
			respbody httpRespProtoPingpongMethodTestMethod
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

		respbody.Success, err = c.methods.TestMethod(r.Context(), reqbody.String, reqbody.Bool, reqbody.Int64, reqbody.Int, reqbody.Float, reqbody.Double, reqbody.Datetime)
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

// The HTTPEchoClient type is a HTTP client for the echo protocol.
type HTTPEchoClient struct {
	client  *HTTPClient
	baseURL string
}

// NewHTTPEchoClient(c *HTTPClient) creates a new HTTP client for the echo protocol.
func NewHTTPEchoClient(c *HTTPClient) *HTTPEchoClient {
	return &HTTPEchoClient{
		client:  c,
		baseURL: "echo",
	}
}

func (c *HTTPEchoClient) newRequest(method, url string, body io.Reader) (*http.Request, error) {
	return c.client.newRequest(method, c.baseURL+"/"+url, body)
}

type httpReqProtoEchoMethodEcho struct {
	Input     string                      `json:"input"`
	Names     []string                    `json:"names"`
	Values    map[string]map[string]int64 `json:"values"`
	Values2   map[string]int64            `json:"values2"`
	Something EchoThing                   `json:"something"`
	Mytime    time.Time                   `json:"mytime"`
	Id        uuid.UUID                   `json:"id"`
}

type httpRespProtoEchoMethodEcho struct {
	Output    string    `json:"output"`
	OuputTime time.Time `json:"ouputTime,omitempty"`
}

// The HTTPEchoClient type is a HTTP client for the Echo protocol.
type HTTPEchoClient struct {
	client  *HTTPClient
	baseURL string
}

// NewHTTPEchoClient(c *HTTPClient) creates a new HTTP client for the Echo protocol.
func NewHTTPEchoClient(c *HTTPClient) *HTTPEchoClient {
	return &HTTPEchoClient{
		client:  c,
		baseURL: "Echo",
	}
}

func (c *HTTPEchoClient) newRequest(method, url string, body io.Reader) (*http.Request, error) {
	return c.client.newRequest(method, c.baseURL+"/"+url, body)
}

type httpRespProtoEchoMethodPing struct {
	Output string `json:"output"`
}

// The HTTPPingClient type is a HTTP client for the Ping protocol.
type HTTPPingClient struct {
	client  *HTTPClient
	baseURL string
}

// NewHTTPPingClient(c *HTTPClient) creates a new HTTP client for the Ping protocol.
func NewHTTPPingClient(c *HTTPClient) *HTTPPingClient {
	return &HTTPPingClient{
		client:  c,
		baseURL: "Ping",
	}
}

func (c *HTTPPingClient) newRequest(method, url string, body io.Reader) (*http.Request, error) {
	return c.client.newRequest(method, c.baseURL+"/"+url, body)
}

type httpReqProtoEchoMethodByteTest struct {
	Input []byte `json:"input"`
}

type httpRespProtoEchoMethodByteTest struct {
	Output []byte `json:"output"`
}

// The HTTPByteTestClient type is a HTTP client for the ByteTest protocol.
type HTTPByteTestClient struct {
	client  *HTTPClient
	baseURL string
}

// NewHTTPByteTestClient(c *HTTPClient) creates a new HTTP client for the ByteTest protocol.
func NewHTTPByteTestClient(c *HTTPClient) *HTTPByteTestClient {
	return &HTTPByteTestClient{
		client:  c,
		baseURL: "ByteTest",
	}
}

func (c *HTTPByteTestClient) newRequest(method, url string, body io.Reader) (*http.Request, error) {
	return c.client.newRequest(method, c.baseURL+"/"+url, body)
}

// HTTPEchoServer creates a new HTTPServer for the echo protocol.
func HTTPEchoServer(methods EchoProtocol) HTTPServer {
	return &httpCallServerForEcho{methods: methods}
}

type httpCallServerForEcho struct {
	methods EchoProtocol
}

func (c *httpCallServerForEcho) RegisterToMux(m *http.ServeMux) {
	m.HandleFunc("/echo/echo", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var (
			err      error
			b        []byte
			reqbody  httpReqProtoEchoMethodEcho
			respbody httpRespProtoEchoMethodEcho
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

		respbody.Output, respbody.OuputTime, err = c.methods.Echo(r.Context(), reqbody.Input, reqbody.Names, reqbody.Values, reqbody.Values2, reqbody.Something, reqbody.Mytime, reqbody.Id)
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
	m.HandleFunc("/echo/ping", func(w http.ResponseWriter, r *http.Request) {
		var (
			err error
			b   []byte

			respbody httpRespProtoEchoMethodPing
		)

		b, err = ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if len(b) > 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		respbody.Output, err = c.methods.Ping(r.Context())
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
	m.HandleFunc("/echo/bytetest", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var (
			err      error
			b        []byte
			reqbody  httpReqProtoEchoMethodByteTest
			respbody httpRespProtoEchoMethodByteTest
		)

		b, err = ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		reqbody.Input = b

		respbody.Output, err = c.methods.ByteTest(r.Context(), reqbody.Input)
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

		b = respbody.Output

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(b)
	})
}
