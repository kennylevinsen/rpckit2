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
	HttpReqUsername string `json:"username"`
	HttpReqPassword string `json:"password"`
}

type httpRespProtoPingpongMethodAuthenticate struct {
	HttpRespSuccess bool `json:"success"`
}

func (c *HTTPPingpongClient) Authenticate(
	ctx context.Context,
	reqUsername string,
	reqPassword string,
) (
	respSuccess bool,
	err error,
) {
	var (
		b        []byte
		req      *http.Request
		resp     *http.Response
		reqbody  httpReqProtoPingpongMethodAuthenticate
		respbody httpRespProtoPingpongMethodAuthenticate
	)
	reqbody = httpReqProtoPingpongMethodAuthenticate{
		HttpReqUsername: reqUsername,
		HttpReqPassword: reqPassword,
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

	respSuccess = respbody.HttpRespSuccess
	return
}

type httpReqProtoPingpongMethodPingWithReply struct {
	HttpReqName string `json:"name"`
}

type httpRespProtoPingpongMethodPingWithReply struct {
	HttpRespGreeting string `json:"greeting"`
}

func (c *HTTPPingpongClient) PingWithReply(
	ctx context.Context,
	reqName string,
) (
	respGreeting string,
	err error,
) {
	var (
		b        []byte
		req      *http.Request
		resp     *http.Response
		reqbody  httpReqProtoPingpongMethodPingWithReply
		respbody httpRespProtoPingpongMethodPingWithReply
	)
	reqbody = httpReqProtoPingpongMethodPingWithReply{
		HttpReqName: reqName,
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

	respGreeting = respbody.HttpRespGreeting
	return
}

type httpReqProtoPingpongMethodTestMethod struct {
	HttpReqString string  `json:"string"`
	HttpReqBool   bool    `json:"bool"`
	HttpReqInt64  int64   `json:"int64"`
	HttpReqInt    int64   `json:"int"`
	HttpReqFloat  float32 `json:"float"`
	HttpReqDouble float64 `json:"double"`
}

type httpRespProtoPingpongMethodTestMethod struct {
	HttpRespSuccess bool `json:"success"`
}

func (c *HTTPPingpongClient) TestMethod(
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
) {
	var (
		b        []byte
		req      *http.Request
		resp     *http.Response
		reqbody  httpReqProtoPingpongMethodTestMethod
		respbody httpRespProtoPingpongMethodTestMethod
	)
	reqbody = httpReqProtoPingpongMethodTestMethod{
		HttpReqString: reqString,
		HttpReqBool:   reqBool,
		HttpReqInt64:  reqInt64,
		HttpReqInt:    reqInt,
		HttpReqFloat:  reqFloat,
		HttpReqDouble: reqDouble,
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

	respSuccess = respbody.HttpRespSuccess
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

		respbody.HttpRespSuccess, err = c.methods.Authenticate(
			r.Context(),
			reqbody.HttpReqUsername,
			reqbody.HttpReqPassword,
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

		respbody.HttpRespGreeting, err = c.methods.PingWithReply(
			r.Context(),
			reqbody.HttpReqName,
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

		respbody.HttpRespSuccess, err = c.methods.TestMethod(
			r.Context(),
			reqbody.HttpReqString,
			reqbody.HttpReqBool,
			reqbody.HttpReqInt64,
			reqbody.HttpReqInt,
			reqbody.HttpReqFloat,
			reqbody.HttpReqDouble,
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
	HttpReqInput  string           `json:"input"`
	HttpReqNames  []string         `json:"names"`
	HttpReqValues map[string]int64 `json:"values"`
}

type httpRespProtoEchoMethodEcho struct {
	HttpRespOutput string `json:"output"`
}

func (c *HTTPEchoClient) Echo(
	ctx context.Context,
	reqInput string,
	reqNames []string,
	reqValues map[string]int64,
) (
	respOutput string,
	err error,
) {
	var (
		b        []byte
		req      *http.Request
		resp     *http.Response
		reqbody  httpReqProtoEchoMethodEcho
		respbody httpRespProtoEchoMethodEcho
	)
	reqbody = httpReqProtoEchoMethodEcho{
		HttpReqInput:  reqInput,
		HttpReqNames:  reqNames,
		HttpReqValues: reqValues,
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

	respOutput = respbody.HttpRespOutput
	return
}

type httpReqProtoEchoMethodPing struct {
}

type httpRespProtoEchoMethodPing struct {
	HttpRespOutput string `json:"output"`
}

func (c *HTTPEchoClient) Ping(
	ctx context.Context,
) (
	respOutput string,
	err error,
) {
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
	respOutput = respbody.HttpRespOutput
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

		respbody.HttpRespOutput, err = c.methods.Echo(
			r.Context(),
			reqbody.HttpReqInput,
			reqbody.HttpReqNames,
			reqbody.HttpReqValues,
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

		respbody.HttpRespOutput, err = c.methods.Ping(
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
