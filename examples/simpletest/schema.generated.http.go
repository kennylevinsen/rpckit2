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
	Vinteger int64       `json:"vinteger"`
	Vint64   int64       `json:"vint64"`
	Vfloat   float32     `json:"vfloat"`
	Vdouble  float64     `json:"vdouble"`
	Vbool    bool        `json:"vbool"`
	Vstring  string      `json:"vstring"`
	Vbytes   []byte      `json:"vbytes"`
	Channels ChannelInfo `json:"channels"`
}

type httpRespProtoPingpongMethodSimpleTest struct {
	Vinteger int64       `json:"vinteger"`
	Vint64   int64       `json:"vint64"`
	Vfloat   float32     `json:"vfloat"`
	Vdouble  float64     `json:"vdouble"`
	Vbool    bool        `json:"vbool"`
	Vstring  string      `json:"vstring"`
	Vbytes   []byte      `json:"vbytes"`
	Channels ChannelInfo `json:"channels"`
}

// The simplest of tests
func (c *HTTPPingpongClient) SimpleTest(ctx context.Context, reqVinteger int64, reqVint64 int64, reqVfloat float32, reqVdouble float64, reqVbool bool, reqVstring string, reqVbytes []byte, reqChannels ChannelInfo) (respVinteger int64, respVint64 int64, respVfloat float32, respVdouble float64, respVbool bool, respVstring string, respVbytes []byte, respChannels ChannelInfo, err error) {
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
		Channels: reqChannels,
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
	respChannels = respbody.Channels

	return
}

type httpReqProtoPingpongMethodArrayTest struct {
	Vinteger []int64       `json:"vinteger"`
	Vint64   []int64       `json:"vint64"`
	Vfloat   []float32     `json:"vfloat"`
	Vdouble  []float64     `json:"vdouble"`
	Vbool    []bool        `json:"vbool"`
	Vstring  []string      `json:"vstring"`
	Vbytes   [][]byte      `json:"vbytes"`
	Channels []ChannelInfo `json:"channels"`
}

type httpRespProtoPingpongMethodArrayTest struct {
	Vinteger []int64       `json:"vinteger"`
	Vint64   []int64       `json:"vint64"`
	Vfloat   []float32     `json:"vfloat"`
	Vdouble  []float64     `json:"vdouble"`
	Vbool    []bool        `json:"vbool"`
	Vstring  []string      `json:"vstring"`
	Vbytes   [][]byte      `json:"vbytes"`
	Channels []ChannelInfo `json:"channels"`
}

// The simplest of tests, but with arrays
func (c *HTTPPingpongClient) ArrayTest(ctx context.Context, reqVinteger []int64, reqVint64 []int64, reqVfloat []float32, reqVdouble []float64, reqVbool []bool, reqVstring []string, reqVbytes [][]byte, reqChannels []ChannelInfo) (respVinteger []int64, respVint64 []int64, respVfloat []float32, respVdouble []float64, respVbool []bool, respVstring []string, respVbytes [][]byte, respChannels []ChannelInfo, err error) {
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
		Channels: reqChannels,
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
	respChannels = respbody.Channels

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

		respbody.Vinteger, respbody.Vint64, respbody.Vfloat, respbody.Vdouble, respbody.Vbool, respbody.Vstring, respbody.Vbytes, respbody.Channels, err = c.methods.SimpleTest(r.Context(), reqbody.Vinteger, reqbody.Vint64, reqbody.Vfloat, reqbody.Vdouble, reqbody.Vbool, reqbody.Vstring, reqbody.Vbytes, reqbody.Channels)
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

		respbody.Vinteger, respbody.Vint64, respbody.Vfloat, respbody.Vdouble, respbody.Vbool, respbody.Vstring, respbody.Vbytes, respbody.Channels, err = c.methods.ArrayTest(r.Context(), reqbody.Vinteger, reqbody.Vint64, reqbody.Vfloat, reqbody.Vdouble, reqbody.Vbool, reqbody.Vstring, reqbody.Vbytes, reqbody.Channels)
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

// The HTTPBackendClient type is a HTTP client for the Backend protocol.
type HTTPBackendClient struct {
	client  *HTTPClient
	baseURL string
}

// NewHTTPBackendClient(c *HTTPClient) creates a new HTTP client for the Backend protocol.
func NewHTTPBackendClient(c *HTTPClient) *HTTPBackendClient {
	return &HTTPBackendClient{
		client:  c,
		baseURL: "Backend",
	}
}

func (c *HTTPBackendClient) newRequest(method, url string, body io.Reader) (*http.Request, error) {
	return c.client.newRequest(method, c.baseURL+"/"+url, body)
}

type httpReqProtoBackendMethodAuthenticate struct {
	Token string `json:"token"`
	Email string `json:"email"`
}

type httpRespProtoBackendMethodAuthenticate struct {
	Token string `json:"token"`
}

// Authenticate using either (token or e-mail).
func (c *HTTPBackendClient) Authenticate(ctx context.Context, reqToken string, reqEmail string) (respToken string, err error) {
	var (
		b        []byte
		req      *http.Request
		resp     *http.Response
		reqbody  httpReqProtoBackendMethodAuthenticate
		respbody httpRespProtoBackendMethodAuthenticate
	)

	reqbody = httpReqProtoBackendMethodAuthenticate{
		Token: reqToken,
		Email: reqEmail,
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
	respToken = respbody.Token

	return
}

type httpRespProtoBackendMethodListChannels struct {
	Greeting string        `json:"greeting"`
	Channels []ChannelInfo `json:"channels"`
}

// List my active channels
func (c *HTTPBackendClient) ListChannels(ctx context.Context) (respGreeting string, respChannels []ChannelInfo, err error) {
	var (
		b    []byte
		req  *http.Request
		resp *http.Response

		respbody httpRespProtoBackendMethodListChannels
	)

	if req, err = c.newRequest("GET", "listchannels", bytes.NewReader(b)); err != nil {
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
	respChannels = respbody.Channels

	return
}

type httpReqProtoBackendMethodSetAvatar struct {
	Image []byte `json:"image"`
}

// Set the avatar for the currently authenticated user
func (c *HTTPBackendClient) SetAvatar(ctx context.Context, reqImage []byte) (err error) {
	var (
		b    []byte
		req  *http.Request
		resp *http.Response
	)

	b = reqImage

	if req, err = c.newRequest("POST", "setavatar", bytes.NewReader(b)); err != nil {
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
	return
}

type httpReqProtoBackendMethodSetName struct {
	Name string `json:"name"`
}

// Set the name of the current user
func (c *HTTPBackendClient) SetName(ctx context.Context, reqName string) (err error) {
	var (
		b       []byte
		req     *http.Request
		resp    *http.Response
		reqbody httpReqProtoBackendMethodSetName
	)

	reqbody = httpReqProtoBackendMethodSetName{
		Name: reqName,
	}

	if b, err = json.Marshal(&reqbody); err != nil {
		return
	}

	if req, err = c.newRequest("POST", "setname", bytes.NewReader(b)); err != nil {
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
	return
}

type httpReqProtoBackendMethodRegisterNotifications struct {
	Name       string `json:"name"`
	Devicetype string `json:"devicetype"`
}

// Register for push notifications
func (c *HTTPBackendClient) RegisterNotifications(ctx context.Context, reqName string, reqDevicetype string) (err error) {
	var (
		b       []byte
		req     *http.Request
		resp    *http.Response
		reqbody httpReqProtoBackendMethodRegisterNotifications
	)

	reqbody = httpReqProtoBackendMethodRegisterNotifications{
		Name:       reqName,
		Devicetype: reqDevicetype,
	}

	if b, err = json.Marshal(&reqbody); err != nil {
		return
	}

	if req, err = c.newRequest("POST", "registernotifications", bytes.NewReader(b)); err != nil {
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
	return
}

// HTTPBackendServer creates a new HTTPServer for the Backend protocol.
func HTTPBackendServer(methods BackendProtocol) HTTPServer {
	return &httpCallServerForBackend{methods: methods}
}

type httpCallServerForBackend struct {
	methods BackendProtocol
}

func (c *httpCallServerForBackend) RegisterToMux(m *http.ServeMux) {
	m.HandleFunc("/Backend/authenticate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var (
			err      error
			b        []byte
			reqbody  httpReqProtoBackendMethodAuthenticate
			respbody httpRespProtoBackendMethodAuthenticate
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

		respbody.Token, err = c.methods.Authenticate(r.Context(), reqbody.Token, reqbody.Email)
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
	m.HandleFunc("/Backend/listchannels", func(w http.ResponseWriter, r *http.Request) {
		var (
			err error
			b   []byte

			respbody httpRespProtoBackendMethodListChannels
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

		respbody.Greeting, respbody.Channels, err = c.methods.ListChannels(r.Context())
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
	m.HandleFunc("/Backend/setavatar", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var (
			err     error
			b       []byte
			reqbody httpReqProtoBackendMethodSetAvatar
		)

		b, err = ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		reqbody.Image = b

		err = c.methods.SetAvatar(r.Context(), reqbody.Image)
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
		b = nil

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(b)
	})
	m.HandleFunc("/Backend/setname", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var (
			err     error
			b       []byte
			reqbody httpReqProtoBackendMethodSetName
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

		err = c.methods.SetName(r.Context(), reqbody.Name)
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
		b = nil

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(b)
	})
	m.HandleFunc("/Backend/registernotifications", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var (
			err     error
			b       []byte
			reqbody httpReqProtoBackendMethodRegisterNotifications
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

		err = c.methods.RegisterNotifications(r.Context(), reqbody.Name, reqbody.Devicetype)
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
		b = nil

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(b)
	})
}
