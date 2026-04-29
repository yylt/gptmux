package ds

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type testDoer func(*http.Request) (*http.Response, error)

func (f testDoer) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestPostJSONWithStatusParses422Body(t *testing.T) {
	client := &Client{
		regular: testDoer(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusUnprocessableEntity,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"code":422,"msg":"bad email format"}`)),
				Request:    req,
			}, nil
		}),
		fallback: &http.Client{},
	}

	body, status, err := client.postJSONWithStatus(context.Background(), LoginURL, baseHeaders, NewLoginPayload("a@example.com", "p"))
	if err != nil {
		t.Fatalf("postJSONWithStatus error: %v", err)
	}
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d want=%d", status, http.StatusUnprocessableEntity)
	}
	if got := intFrom(body["code"]); got != 422 {
		t.Fatalf("expected code 422, got %#v", body)
	}
	if got, _ := body["msg"].(string); got != "bad email format" {
		t.Fatalf("unexpected msg: %#v", body["msg"])
	}
}

func TestCallCompletionIncludesPassThroughAndPowHeader(t *testing.T) {
	t.Helper()
	client := &Client{
		stream: testDoer(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("x-ds-pow-response"); got != "pow-token" {
				t.Fatalf("unexpected pow header: %q", got)
			}
			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			var body map[string]any
			if err := json.Unmarshal(bodyBytes, &body); err != nil {
				t.Fatalf("unmarshal body: %v", err)
			}
			if got := body["chat_session_id"]; got != "session-1" {
				t.Fatalf("unexpected chat_session_id: %#v", got)
			}
			if got := body["extra_field"]; got != "extra-value" {
				t.Fatalf("unexpected extra_field: %#v", got)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("ok")),
				Request:    req,
			}, nil
		}),
		fallbackS: &http.Client{},
	}

	resp, err := client.CallCompletion(context.Background(), "token", CompletionPayload{
		ChatSessionID: "session-1",
		Prompt:        "hello",
		PassThrough: map[string]any{
			"extra_field": "extra-value",
		},
	}, "pow-token")
	if err != nil {
		t.Fatalf("CallCompletion error: %v", err)
	}
	_ = resp.Body.Close()
}

func TestCallCompletionRetriesNon200(t *testing.T) {
	attempts := 0
	client := &Client{
		stream: testDoer(func(req *http.Request) (*http.Response, error) {
			attempts++
			status := http.StatusUnprocessableEntity
			body := `{"code":422,"msg":"bad request"}`
			if attempts == 3 {
				status = http.StatusOK
				body = "ok"
			}
			return &http.Response{
				StatusCode: status,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
				Request:    req,
			}, nil
		}),
		fallbackS: &http.Client{},
	}

	resp, err := client.CallCompletion(context.Background(), "token", CompletionPayload{
		ChatSessionID: "session-1",
		Prompt:        "hello",
	}, "")
	if err != nil {
		t.Fatalf("CallCompletion error: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts=%d want=3", attempts)
	}
	_ = resp.Body.Close()
}
