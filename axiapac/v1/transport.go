package v1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type Response struct {
	Data []byte
}

// Transport handles low-level HTTP and authentication
type Transport struct {
	BaseURL    string
	AuthToken  string
	HTTPClient *http.Client
}

// NewTransport creates a transport with base URL and auth
func NewTransport(baseURL, token string) *Transport {
	return &Transport{
		BaseURL:    baseURL,
		AuthToken:  token,
		HTTPClient: &http.Client{},
	}
}

// helper: build full URL with query params
func (t *Transport) buildURL(path string, query map[string]string) string {
	u, _ := url.Parse(t.BaseURL + path)
	q := u.Query()
	for k, v := range query {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// Post sends a POST request with JSON body
func (t *Transport) Post(path string, data any, query map[string]string) (*Response, error) {
	fullURL := t.buildURL(path, query)

	body, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, fullURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if t.AuthToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.AuthToken))
	}

	resp, err := t.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("POST %s failed with status code %d: %s", path, resp.StatusCode, string(b))
	}

	resdata, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return &Response{
		Data: resdata,
	}, nil
}

// Get sends a GET request
func (t *Transport) Get(path string, query map[string]string) (*http.Response, error) {
	fullURL := t.buildURL(path, query)

	req, err := http.NewRequest(http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}

	if t.AuthToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.AuthToken))
	}

	return t.HTTPClient.Do(req)
}
