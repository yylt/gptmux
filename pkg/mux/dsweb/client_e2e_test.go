package ds

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestClientEmailPasswordE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DeepSeek e2e test in short mode")
	}

	email := strings.TrimSpace(os.Getenv("DS2API_DEEPSEEK_EMAIL"))
	password := strings.TrimSpace(os.Getenv("DS2API_DEEPSEEK_PASSWORD"))
	if email == "" || password == "" {
		t.Skip("set DS2API_DEEPSEEK_EMAIL and DS2API_DEEPSEEK_PASSWORD to run this test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	client := NewClient()
	token, err := client.Login(ctx, email, password)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	sessionID, err := client.CreateSession(ctx, token)
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}
	if sessionID == "" {
		t.Fatal("create session returned empty session id")
	}

	powHeader, err := client.GetPowHeader(ctx, token, "/api/v0/chat/completion")
	if err != nil {
		t.Fatalf("pow header failed: %v", err)
	}

	completionPayload := NewCompletionPayload(sessionID, "hi")
	resp, err := client.CallCompletion(ctx, token, completionPayload, powHeader)
	if err != nil {
		t.Fatalf("call completion failed: %v", err)
	}
	if resp == nil || resp.Body == nil {
		t.Fatal("call completion returned empty response")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		t.Fatalf("unexpected completion status: %d", resp.StatusCode)
	}
	if resp.Header.Get("Content-Type") == "" {
		t.Fatal("expected completion response headers")
	}

	list, hasMore, err := client.ListSession(ctx, token)
	if err != nil {
		t.Fatalf("list session failed: %v", err)
	}
	t.Logf("list session parsed result: sessions=%d hasMore=%t list=%+v", len(list), hasMore, list)

	result, err := client.DeleteSession(ctx, token, sessionID)
	if err != nil {
		t.Fatalf("delete session failed: %v", err)
	}
	if !result.Success {
		t.Fatalf("delete session reported failure for %s", sessionID)
	}
}
