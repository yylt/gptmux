package util

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"time"
)

type logTransport struct {
	tr    http.RoundTripper
	debug bool
}

func NewDebugHTTPClient(proxy string, debug bool) *http.Client {
	var (
		tr = &http.Transport{
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
	)
	if proxy != "" {
		proxy, err := ParseUrl(proxy)
		if err != nil {
			return nil
		}
		tr.Proxy = http.ProxyURL(proxy)
	}
	return &http.Client{
		Transport: &logTransport{
			tr:    tr,
			debug: debug,
		},
	}
}

// RoundTrip logs the request and response with full contents using httputil.DumpRequest and httputil.DumpResponse.
func (t *logTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.debug {
		dump, err := httputil.DumpRequestOut(req, true)
		if err != nil {
			return nil, err
		}
		fmt.Println(string(dump)) //nolint:forbidigo
	}
	resp, err := t.tr.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if t.debug {
		dump, err := httputil.DumpResponse(resp, true)
		if err != nil {
			return nil, err
		}
		fmt.Println(string(dump)) //nolint:forbidigo
	}
	return resp, nil
}
