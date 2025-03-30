package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	client := NewHTTPbinAPIClient()
	var buf bytes.Buffer
	if err := client.Json(context.Background(), &buf); err != nil {
		log.Fatal(err)
	}
	io.Copy(os.Stdout, &buf)
}

type HTTPbinAPIClient struct {
	APIClient
}

func NewHTTPbinAPIClient() *HTTPbinAPIClient {
	apiClient := NewAPIClient("HTTPBin", nil)
	apiClient.BaseURL = "https://httpbin.org"
	return &HTTPbinAPIClient{
		APIClient: *apiClient,
	}
}
func (client *HTTPbinAPIClient) Json(ctx context.Context, w io.Writer) error {
	resp, err := client.Get(ctx, "/json")
	// You can augment errors with more information
	if err != nil {
		return err
	}

	if err := client.FailForStatusCode(resp); err != nil {
		return err
	}
	defer resp.Body.Close()

	resp.Write(w)
	return nil
}

type APIClient struct {
	Name    string
	BaseURL string
	Timeout time.Duration
	Logger  *slog.Logger
}

func NewAPIClient(name string, logger *slog.Logger) *APIClient {
	if logger == nil {
		logger = slog.Default()
	}
	return &APIClient{
		Name:   name,
		Logger: logger,
	}
}

func (client *APIClient) String() string {
	return client.repr()
}

type requestOption struct {
	timeout time.Duration
	body    io.Reader
}

type RequestOption func(option *requestOption)

func WithTimeout(timeout time.Duration) RequestOption {
	return func(option *requestOption) {
		option.timeout = timeout
	}
}

func WithBody(body io.Reader) RequestOption {
	return func(option *requestOption) {
		option.body = body
	}
}

func (client *APIClient) Post(ctx context.Context, path string, body io.Reader, options ...RequestOption) (*http.Response, error) {
	return client.Request(ctx, string(http.MethodPost), path)
}

func (client *APIClient) Patch(ctx context.Context, path string, body io.Reader, options ...RequestOption) (*http.Response, error) {
	return client.Request(ctx, string(http.MethodPatch), path)
}

func (client *APIClient) Put(ctx context.Context, path string, body io.Reader, options ...RequestOption) (*http.Response, error) {
	return client.Request(ctx, string(http.MethodPut), path)
}

func (client *APIClient) Get(ctx context.Context, path string, options ...RequestOption) (*http.Response, error) {
	return client.Request(ctx, string(http.MethodGet), path)
}

func (client *APIClient) Head(ctx context.Context, path string, options ...RequestOption) (*http.Response, error) {
	return client.Request(ctx, string(http.MethodHead), path)
}

func (client *APIClient) Options(ctx context.Context, path string, options ...RequestOption) (*http.Response, error) {
	return client.Request(ctx, string(http.MethodOptions), path)
}

func (client *APIClient) Delete(ctx context.Context, path string, options ...RequestOption) (*http.Response, error) {
	return client.Request(ctx, string(http.MethodDelete), path)
}

func (client *APIClient) Request(ctx context.Context, method string, path string, options ...RequestOption) (*http.Response, error) {

	// build options
	opts := &requestOption{}
	for _, opt := range options {
		opt(opts)
	}

	if opts.timeout != 0 {
		ctx, _ = context.WithTimeout(ctx, opts.timeout)
	}

	// build URL
	url, err := client.buildURL(path)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, url, opts.body)
	if err != nil {
		return nil, err
	}

	c := http.Client{Timeout: client.Timeout}

	resp, err := c.Do(req)
	if err != nil {
		client.trackResponse(req, nil, err)
		return nil, err
	}

	client.trackResponse(req, resp, nil)
	return resp, nil
}

func (client *APIClient) FailForStatusCode(resp *http.Response) error {
	// you can provide a more structured errors here, something line type APIError struct {}
	if resp.StatusCode >= 400 {
		return errors.New(resp.Status)
	}
	return nil
}

func (client *APIClient) buildURL(path string) (string, error) {
	if !strings.HasPrefix(path, "/") {
		return path, nil
	}
	if strings.HasPrefix(path, "/") && client.BaseURL == "" {
		return "", errors.New("You have to provide the baseURL")
	}
	baseURL, _ := strings.CutSuffix(client.BaseURL, "/")
	path, _ = strings.CutPrefix(path, "/")
	return strings.Join([]string{baseURL, path}, "/"), nil
}

func (client *APIClient) trackResponse(req *http.Request, resp *http.Response, err error) {
	log := client.Logger.Error
	if resp != nil {
		// Make sure we get the actual request that produces
		// this response
		req = resp.Request

		if resp.StatusCode < 400 {
			log = client.Logger.Info
		}

		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			log = client.Logger.Warn
		}
	}

	defaultStatusCode := 500

	status := ""
	if resp != nil {
		status = resp.Status
	} else {
		status = fmt.Sprintf("%d %s", defaultStatusCode, err.Error())
	}

	log(client.repr(), "method", req.Method, "path", req.URL.Path, "status", status)
}

func (client *APIClient) repr() string {
	name := client.Name
	if client.BaseURL != "" {
		name = fmt.Sprintf("%s (baseURL: %s)", client.Name, client.BaseURL)
	}
	return name
}
