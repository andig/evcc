package request

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/andig/evcc/util"
)

// Timeout is the default request timeout used by the Helper
var Timeout = 10 * time.Second

// Helper provides utility primitives
type Helper struct {
	*http.Client
}

type helperOption func(*Helper)

// WithMetricsPush enables request helper to push metrics
func WithMetricsPush(h *Helper) {
	h.Transport.(*RoundTripper).MetricsPush = true
}

// NewHelper creates http helper for simplified PUT GET logic
func NewHelper(log *util.Logger, options ...helperOption) *Helper {
	r := &Helper{
		Client: &http.Client{
			Timeout:   Timeout,
			Transport: NewTripper(log, http.DefaultTransport),
		},
	}

	// apply options
	for _, o := range options {
		o(r)
	}

	return r
}

// DoBody executes HTTP request and returns the response body
func (r *Helper) DoBody(req *http.Request) ([]byte, error) {
	resp, err := r.Do(req)
	var body []byte
	if err == nil {
		body, err = ReadBody(resp)
	}
	return body, err
}

// GetBody executes HTTP GET request and returns the response body
func (r *Helper) GetBody(url string) ([]byte, error) {
	resp, err := r.Get(url)
	var body []byte
	if err == nil {
		body, err = ReadBody(resp)
	}
	return body, err
}

// decodeJSON reads HTTP response and decodes JSON body if error is nil
func decodeJSON(resp *http.Response, res interface{}) error {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return StatusError{resp: resp}
	}

	return json.NewDecoder(resp.Body).Decode(&res)
}

// DoJSON executes HTTP request and decodes JSON response. It returns a StatusError on response codes other than HTTP 2xx.
func (r *Helper) DoJSON(req *http.Request, res interface{}) error {
	resp, err := r.Do(req)
	if err == nil {
		defer resp.Body.Close()
		err = decodeJSON(resp, &res)
	}
	return err
}

// GetJSON executes HTTP GET request and decodes JSON response. It returns a StatusError on response codes other than HTTP 2xx.
func (r *Helper) GetJSON(url string, res interface{}) error {
	resp, err := r.Get(url)
	if err == nil {
		defer resp.Body.Close()
		err = decodeJSON(resp, &res)
	}
	return err
}
