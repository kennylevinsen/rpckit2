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

type PingpongClient struct {
	client  *HTTPClient
	baseURL string
}

func NewPingpongClient(c *HTTPClient) *PingpongClient {
	return &PingpongClient{
		client:  c,
		baseURL: "pingpong",
	}
}

func (c *PingpongClient) NewRequest(method, url string, body io.Reader) (*http.Request, error) {
	return c.client.NewRequest(method, c.baseURL+"/"+url, body)
}

type httprequest_Pingpong_Authenticate struct {
	_username string `json:"username"`
	_password string `json:"password"`
}

type httpresponse_Pingpong_Authenticate struct {
	_success bool `json:"success"`
}

func (c *PingpongClient) Authenticate(
	ctx context.Context,
	in_username string,
	in_password string,
) (
	out_success bool,
	err error,
) {
	var (
		b        []byte
		req      *http.Request
		resp     *http.Response
		reqbody  httprequest_Pingpong_Authenticate
		respbody httpresponse_Pingpong_Authenticate
	)
	reqbody = httprequest_Pingpong_Authenticate{
		_username: in_username,
		_password: in_password,
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

	out_success = respbody._success
	return
}

type httprequest_Pingpong_PingWithReply struct {
	_name string `json:"name"`
}

type httpresponse_Pingpong_PingWithReply struct {
	_greeting string `json:"greeting"`
}

func (c *PingpongClient) PingWithReply(
	ctx context.Context,
	in_name string,
) (
	out_greeting string,
	err error,
) {
	var (
		b        []byte
		req      *http.Request
		resp     *http.Response
		reqbody  httprequest_Pingpong_PingWithReply
		respbody httpresponse_Pingpong_PingWithReply
	)
	reqbody = httprequest_Pingpong_PingWithReply{
		_name: in_name,
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

	out_greeting = respbody._greeting
	return
}

type httprequest_Pingpong_TestMethod struct {
	_string string  `json:"string"`
	_bool   bool    `json:"bool"`
	_int64  int64   `json:"int64"`
	_int    int64   `json:"int"`
	_float  float32 `json:"float"`
	_double float64 `json:"double"`
}

type httpresponse_Pingpong_TestMethod struct {
	_success bool `json:"success"`
}

func (c *PingpongClient) TestMethod(
	ctx context.Context,
	in_string string,
	in_bool bool,
	in_int64 int64,
	in_int int64,
	in_float float32,
	in_double float64,
) (
	out_success bool,
	err error,
) {
	var (
		b        []byte
		req      *http.Request
		resp     *http.Response
		reqbody  httprequest_Pingpong_TestMethod
		respbody httpresponse_Pingpong_TestMethod
	)
	reqbody = httprequest_Pingpong_TestMethod{
		_string: in_string,
		_bool:   in_bool,
		_int64:  in_int64,
		_int:    in_int,
		_float:  in_float,
		_double: in_double,
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

	out_success = respbody._success
	return
}

type EchoClient struct {
	client  *HTTPClient
	baseURL string
}

func NewEchoClient(c *HTTPClient) *EchoClient {
	return &EchoClient{
		client:  c,
		baseURL: "echo",
	}
}

func (c *EchoClient) NewRequest(method, url string, body io.Reader) (*http.Request, error) {
	return c.client.NewRequest(method, c.baseURL+"/"+url, body)
}

type httprequest_Echo_Echo struct {
	_input  string           `json:"input"`
	_names  []string         `json:"names"`
	_values map[string]int64 `json:"values"`
}

type httpresponse_Echo_Echo struct {
	_output string `json:"output"`
}

func (c *EchoClient) Echo(
	ctx context.Context,
	in_input string,
	in_names []string,
	in_values map[string]int64,
) (
	out_output string,
	err error,
) {
	var (
		b        []byte
		req      *http.Request
		resp     *http.Response
		reqbody  httprequest_Echo_Echo
		respbody httpresponse_Echo_Echo
	)
	reqbody = httprequest_Echo_Echo{
		_input:  in_input,
		_names:  in_names,
		_values: in_values,
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

	out_output = respbody._output
	return
}

type httprequest_Echo_Ping struct {
}

type httpresponse_Echo_Ping struct {
	_output string `json:"output"`
}

func (c *EchoClient) Ping(
	ctx context.Context,
) (
	out_output string,
	err error,
) {
	var (
		b    []byte
		req  *http.Request
		resp *http.Response

		respbody httpresponse_Echo_Ping
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
	out_output = respbody._output
	return
}
