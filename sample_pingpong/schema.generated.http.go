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

type httpReqProtoPingpongMethodAuthenticate struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type httpRespProtoPingpongMethodAuthenticate struct {
	Success bool `json:"success"`
}

// Authenticate using username and password
func (c *HTTPPingpongClient) Authenticate(ctx context.Context, reqUsername string, reqPassword string) (respSuccess bool, err error) {
	var (
		b        []byte
		req      *http.Request
		resp     *http.Response
		reqbody  httpReqProtoPingpongMethodAuthenticate
		respbody httpRespProtoPingpongMethodAuthenticate
	)

	reqbody = httpReqProtoPingpongMethodAuthenticate{
		Username: reqUsername,
		Password: reqPassword,
	}

	if b, err = json.Marshal(&reqbody); err != nil {
		return
	}

	if req, err = c.newRequest("POST", "authenticate", bytes.NewReader(b)); err != nil {
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
	respSuccess = respbody.Success

	return
}

type httpReqProtoPingpongMethodPingWithReply struct {
	Name string `json:"name"`
}

type httpRespProtoPingpongMethodPingWithReply struct {
	Greeting string `json:"greeting"`
}

// PingWithReply replies with a greeting based on the provided name
func (c *HTTPPingpongClient) PingWithReply(ctx context.Context, reqName string) (respGreeting string, err error) {
	var (
		b        []byte
		req      *http.Request
		resp     *http.Response
		reqbody  httpReqProtoPingpongMethodPingWithReply
		respbody httpRespProtoPingpongMethodPingWithReply
	)

	reqbody = httpReqProtoPingpongMethodPingWithReply{
		Name: reqName,
	}

	if b, err = json.Marshal(&reqbody); err != nil {
		return
	}

	if req, err = c.newRequest("POST", "pingwithreply", bytes.NewReader(b)); err != nil {
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
	respGreeting = respbody.Greeting

	return
}

type httpReqProtoPingpongMethodTestMethod struct {
	String string  `json:"string"`
	Bool   bool    `json:"bool"`
	Int64  int64   `json:"int64"`
	Int    int64   `json:"int"`
	Float  float32 `json:"float"`
	Double float64 `json:"double"`
}

type httpRespProtoPingpongMethodTestMethod struct {
	Success bool `json:"success"`
}

// TestMethod is a simple type test
func (c *HTTPPingpongClient) TestMethod(ctx context.Context, reqString string, reqBool bool, reqInt64 int64, reqInt int64, reqFloat float32, reqDouble float64) (respSuccess bool, err error) {
	var (
		b        []byte
		req      *http.Request
		resp     *http.Response
		reqbody  httpReqProtoPingpongMethodTestMethod
		respbody httpRespProtoPingpongMethodTestMethod
	)

	reqbody = httpReqProtoPingpongMethodTestMethod{
		String: reqString,
		Bool:   reqBool,
		Int64:  reqInt64,
		Int:    reqInt,
		Float:  reqFloat,
		Double: reqDouble,
	}

	if b, err = json.Marshal(&reqbody); err != nil {
		return
	}

	if req, err = c.newRequest("POST", "testmethod", bytes.NewReader(b)); err != nil {
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
	respSuccess = respbody.Success

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
			w.WriteHeader(http.StatusInternalServerError)
			errorbody := struct {
				Error string `json:"error"`
			}{
				Error: err.Error(),
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
			w.WriteHeader(http.StatusInternalServerError)
			errorbody := struct {
				Error string `json:"error"`
			}{
				Error: err.Error(),
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

		respbody.Success, err = c.methods.TestMethod(r.Context(), reqbody.String, reqbody.Bool, reqbody.Int64, reqbody.Int, reqbody.Float, reqbody.Double)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			errorbody := struct {
				Error string `json:"error"`
			}{
				Error: err.Error(),
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
}

type httpRespProtoEchoMethodEcho struct {
	Output string `json:"output"`
}

// Echo is yet another type test
func (c *HTTPEchoClient) Echo(ctx context.Context, reqInput string, reqNames []string, reqValues map[string]map[string]int64, reqValues2 map[string]int64, reqSomething EchoThing) (respOutput string, err error) {
	var (
		b        []byte
		req      *http.Request
		resp     *http.Response
		reqbody  httpReqProtoEchoMethodEcho
		respbody httpRespProtoEchoMethodEcho
	)

	reqbody = httpReqProtoEchoMethodEcho{
		Input:     reqInput,
		Names:     reqNames,
		Values:    reqValues,
		Values2:   reqValues2,
		Something: reqSomething,
	}

	if b, err = json.Marshal(&reqbody); err != nil {
		return
	}

	if req, err = c.newRequest("POST", "echo", bytes.NewReader(b)); err != nil {
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
	respOutput = respbody.Output

	return
}

type httpRespProtoEchoMethodPing struct {
	Output string `json:"output"`
}

// Ping is a simple no-input test
func (c *HTTPEchoClient) Ping(ctx context.Context) (respOutput string, err error) {
	var (
		b    []byte
		req  *http.Request
		resp *http.Response

		respbody httpRespProtoEchoMethodPing
	)

	if req, err = c.newRequest("GET", "ping", bytes.NewReader(b)); err != nil {
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
	respOutput = respbody.Output

	return
}

type httpReqProtoEchoMethodByteTest struct {
	Input []byte `json:"input"`
}

type httpRespProtoEchoMethodByteTest struct {
	Output []byte `json:"output"`
}

// ByteTest is a byte test
func (c *HTTPEchoClient) ByteTest(ctx context.Context, reqInput []byte) (respOutput []byte, err error) {
	var (
		b    []byte
		req  *http.Request
		resp *http.Response
	)

	b = reqInput

	if req, err = c.newRequest("POST", "bytetest", bytes.NewReader(b)); err != nil {
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

	respOutput = b

	return
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

		respbody.Output, err = c.methods.Echo(r.Context(), reqbody.Input, reqbody.Names, reqbody.Values, reqbody.Values2, reqbody.Something)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			errorbody := struct {
				Error string `json:"error"`
			}{
				Error: err.Error(),
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
			w.WriteHeader(http.StatusInternalServerError)
			errorbody := struct {
				Error string `json:"error"`
			}{
				Error: err.Error(),
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
			w.WriteHeader(http.StatusInternalServerError)
			errorbody := struct {
				Error string `json:"error"`
			}{
				Error: err.Error(),
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
