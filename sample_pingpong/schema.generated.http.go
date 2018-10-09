package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"

	"golang.org/x/net/publicsuffix"
)

type HTTPClient struct {
	client  *http.Client
	baseURL string
}

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

func (c *HTTPClient) NewRequest(method, url string, body io.Reader) (*http.Request, error) {
	return http.NewRequest(method, c.baseURL+"/"+url, body)
}

func (c *HTTPClient) Do(req *http.Request) (*http.Response, error) {
	return c.client.Do(req)
}

type HTTPHandler interface {
	RegisterToMux(*http.ServeMux)
}

type HTTPPingpongClient struct {
	client  *HTTPClient
	baseURL string
}

func NewHTTPPingpongClient(c *HTTPClient) *HTTPPingpongClient {
	return &HTTPPingpongClient{
		client:  c,
		baseURL: "pingpong",
	}
}

func (c *HTTPPingpongClient) NewRequest(method, url string, body io.Reader) (*http.Request, error) {
	return c.client.NewRequest(method, c.baseURL+"/"+url, body)
}

type httpReqProtoPingpongMethodAuthenticate struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type httpRespProtoPingpongMethodAuthenticate struct {
	Success bool `json:"success"`
}

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

	if req, err = c.NewRequest("POST", "authenticate", bytes.NewReader(b)); err != nil {
		return
	}

	req = req.WithContext(ctx)
	if resp, err = c.client.client.Do(req); err != nil {
		return
	}

	b, err = ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
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

	if req, err = c.NewRequest("POST", "pingwithreply", bytes.NewReader(b)); err != nil {
		return
	}

	req = req.WithContext(ctx)
	if resp, err = c.client.client.Do(req); err != nil {
		return
	}

	b, err = ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
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

	if req, err = c.NewRequest("POST", "testmethod", bytes.NewReader(b)); err != nil {
		return
	}

	req = req.WithContext(ctx)
	if resp, err = c.client.client.Do(req); err != nil {
		return
	}

	b, err = ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return
	}
	if err = json.Unmarshal(b, &respbody); err != nil {
		return
	}

	respSuccess = respbody.Success
	return
}

func HTTPPingpongHandler(methods PingpongMethods) HTTPHandler {
	return &httpCallHandlerForPingpong{methods: methods}
}

type httpCallHandlerForPingpong struct {
	methods PingpongMethods
}

func (c *httpCallHandlerForPingpong) RegisterToMux(m *http.ServeMux) {
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

		respbody.Success, err = c.methods.Authenticate(
			r.Context(),
			reqbody.Username,
			reqbody.Password,
		)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			// TODO: Add error!
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

		respbody.Greeting, err = c.methods.PingWithReply(
			r.Context(),
			reqbody.Name,
		)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			// TODO: Add error!
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

		respbody.Success, err = c.methods.TestMethod(
			r.Context(),
			reqbody.String,
			reqbody.Bool,
			reqbody.Int64,
			reqbody.Int,
			reqbody.Float,
			reqbody.Double,
		)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			// TODO: Add error!
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

type HTTPEchoClient struct {
	client  *HTTPClient
	baseURL string
}

func NewHTTPEchoClient(c *HTTPClient) *HTTPEchoClient {
	return &HTTPEchoClient{
		client:  c,
		baseURL: "echo",
	}
}

func (c *HTTPEchoClient) NewRequest(method, url string, body io.Reader) (*http.Request, error) {
	return c.client.NewRequest(method, c.baseURL+"/"+url, body)
}

type httpReqProtoEchoMethodEcho struct {
	Input  string           `json:"input"`
	Names  []string         `json:"names"`
	Values map[string]int64 `json:"values"`
}

type httpRespProtoEchoMethodEcho struct {
	Output string `json:"output"`
}

func (c *HTTPEchoClient) Echo(ctx context.Context, reqInput string, reqNames []string, reqValues map[string]int64) (respOutput string, err error) {
	var (
		b        []byte
		req      *http.Request
		resp     *http.Response
		reqbody  httpReqProtoEchoMethodEcho
		respbody httpRespProtoEchoMethodEcho
	)
	reqbody = httpReqProtoEchoMethodEcho{
		Input:  reqInput,
		Names:  reqNames,
		Values: reqValues,
	}

	if b, err = json.Marshal(&reqbody); err != nil {
		return
	}

	if req, err = c.NewRequest("POST", "echo", bytes.NewReader(b)); err != nil {
		return
	}

	req = req.WithContext(ctx)
	if resp, err = c.client.client.Do(req); err != nil {
		return
	}

	b, err = ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return
	}
	if err = json.Unmarshal(b, &respbody); err != nil {
		return
	}

	respOutput = respbody.Output
	return
}

type httpReqProtoEchoMethodPing struct {
}

type httpRespProtoEchoMethodPing struct {
	Output string `json:"output"`
}

func (c *HTTPEchoClient) Ping(ctx context.Context) (respOutput string, err error) {
	var (
		b    []byte
		req  *http.Request
		resp *http.Response

		respbody httpRespProtoEchoMethodPing
	)

	if req, err = c.NewRequest("GET", "ping", bytes.NewReader(b)); err != nil {
		return
	}

	req = req.WithContext(ctx)
	if resp, err = c.client.client.Do(req); err != nil {
		return
	}

	b, err = ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return
	}
	respOutput = respbody.Output
	return
}

func HTTPEchoHandler(methods EchoMethods) HTTPHandler {
	return &httpCallHandlerForEcho{methods: methods}
}

type httpCallHandlerForEcho struct {
	methods EchoMethods
}

func (c *httpCallHandlerForEcho) RegisterToMux(m *http.ServeMux) {
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

		respbody.Output, err = c.methods.Echo(
			r.Context(),
			reqbody.Input,
			reqbody.Names,
			reqbody.Values,
		)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			// TODO: Add error!
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

		respbody.Output, err = c.methods.Ping(
			r.Context(),
		)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			// TODO: Add error!
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
