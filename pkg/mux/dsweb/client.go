package ds

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func cloneStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

type Client struct {
	regular   Doer
	stream    Doer
	fallback  *http.Client
	fallbackS *http.Client
}

type LoginPayload struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	DeviceID string `json:"device_id"`
	OS       string `json:"os"`
}

type CreateSessionPayload struct {
	Agent string `json:"agent"`
}

type DeleteSessionResult struct {
	SessionID string
	Success   bool
}

type DeleteSessionPayload struct {
	ChatSessionID string `json:"chat_session_id"`
}

type SessionInfo struct {
	ID        string
	Title     string
	TitleType string
	Pinned    bool
	UpdatedAt float64
}

type CompletionPayload struct {
	ChatSessionID   string         `json:"chat_session_id"`
	ModelType       string         `json:"model_type"`
	ParentMessageID *int           `json:"parent_message_id"`
	Prompt          string         `json:"prompt"`
	RefFileIDs      []string       `json:"ref_file_ids,omitempty"`
	ThinkingEnabled bool           `json:"thinking_enabled"`
	SearchEnabled   bool           `json:"search_enabled"`
	PassThrough     map[string]any `json:"-"`
}

func NewClient() *Client {
	return &Client{
		regular:   newTransport(60 * time.Second),
		stream:    newTransport(0),
		fallback:  newFallbackClient(60*time.Second, nil),
		fallbackS: newFallbackClient(0, nil),
	}
}

func NewLoginPayload(email, password string) LoginPayload {
	return LoginPayload{
		Email:    strings.TrimSpace(email),
		Password: strings.TrimSpace(password),
		DeviceID: "deepseek_to_api",
		OS:       "android",
	}
}

func NewCreateSessionPayload() CreateSessionPayload {
	return CreateSessionPayload{Agent: "chat"}
}

func NewDeleteSessionPayload(sessionID string) DeleteSessionPayload {
	return DeleteSessionPayload{ChatSessionID: strings.TrimSpace(sessionID)}
}

func NewCompletionPayload(sessionID, prompt string) CompletionPayload {
	return CompletionPayload{
		ChatSessionID: strings.TrimSpace(sessionID),
		ModelType:     "default",
		Prompt:        prompt,
		PassThrough:   map[string]any{},
	}
}

func (c *Client) Login(ctx context.Context, email, password string) (string, error) {
	payload := NewLoginPayload(email, password)
	email = payload.Email
	password = payload.Password
	if email == "" || password == "" {
		return "", errors.New("email and password are required")
	}
	resp, _, err := c.postJSONWithStatus(ctx, LoginURL, baseHeaders, payload)
	if err != nil {
		return "", err
	}
	if code := intFrom(resp["code"]); code != 0 {
		return "", fmt.Errorf("login failed: %v", resp["msg"])
	}
	data, _ := resp["data"].(map[string]any)
	if intFrom(data["biz_code"]) != 0 {
		return "", fmt.Errorf("login failed: %v", data["biz_msg"])
	}
	bizData, _ := data["biz_data"].(map[string]any)
	user, _ := bizData["user"].(map[string]any)
	token, _ := user["token"].(string)
	if strings.TrimSpace(token) == "" {
		return "", errors.New("missing login token")
	}
	return token, nil
}

func (c *Client) CreateSession(ctx context.Context, token string) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", errors.New("token is required")
	}
	resp, _, err := c.postJSONWithStatus(ctx, CreateSessionURL, authHeaders(token), NewCreateSessionPayload())
	if err != nil {
		return "", err
	}
	if code := intFrom(resp["code"]); code != 0 {
		return "", fmt.Errorf("create session failed: %v", resp["msg"])
	}
	data, _ := resp["data"].(map[string]any)
	if intFrom(data["biz_code"]) != 0 {
		return "", fmt.Errorf("create session failed: %v", data["biz_msg"])
	}
	bizData, _ := data["biz_data"].(map[string]any)
	if sessionID := sessionIDFromBizData(bizData); sessionID != "" {
		return sessionID, nil
	}
	return "", errors.New("missing session id")
}

func (c *Client) CallCompletion(ctx context.Context, token string, payload CompletionPayload, powResp string) (*http.Response, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("token is required")
	}
	if strings.TrimSpace(payload.ChatSessionID) == "" {
		return nil, errors.New("chat_session_id is required")
	}
	if strings.TrimSpace(payload.Prompt) == "" {
		return nil, errors.New("prompt is required")
	}
	if strings.TrimSpace(payload.ModelType) == "" {
		payload.ModelType = "default"
	}
	headers := authHeaders(token)
	if strings.TrimSpace(powResp) != "" {
		headers["x-ds-pow-response"] = powResp
	}
	body := completionRequestBody(payload)
	for attempts := 0; attempts < 3; attempts++ {
		resp, err := c.streamPost(ctx, CompletionURL, headers, body, "")
		if err != nil {
			if attempts == 2 {
				return nil, err
			}
			time.Sleep(time.Second)
			continue
		}
		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}
		_ = resp.Body.Close()
		if attempts < 2 {
			time.Sleep(time.Second)
		}
	}
	return nil, errors.New("completion failed")
}

func (c *Client) GetPowHeader(ctx context.Context, token, targetPath string) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", errors.New("token is required")
	}
	targetPath = strings.TrimSpace(targetPath)
	if targetPath == "" {
		targetPath = CompletionTargetPath
	}
	payload := map[string]any{"target_path": targetPath}
	resp, _, err := c.postJSONWithStatus(ctx, CreatePowURL, authHeaders(token), payload)
	if err != nil {
		return "", err
	}
	if code := intFrom(resp["code"]); code != 0 {
		return "", fmt.Errorf("create pow challenge failed: %v", resp["msg"])
	}
	data, _ := resp["data"].(map[string]any)
	if intFrom(data["biz_code"]) != 0 {
		return "", fmt.Errorf("create pow challenge failed: %v", data["biz_msg"])
	}
	bizData, _ := data["biz_data"].(map[string]any)
	challengeMap, _ := bizData["challenge"].(map[string]any)
	if challengeMap == nil {
		return "", errors.New("missing pow challenge")
	}
	challenge := &Challenge{
		Algorithm:  stringFromMap(challengeMap, "algorithm"),
		Challenge:  stringFromMap(challengeMap, "challenge"),
		Salt:       stringFromMap(challengeMap, "salt"),
		Signature:  stringFromMap(challengeMap, "signature"),
		TargetPath: stringFromMap(challengeMap, "target_path"),
		ExpireAt:   int64From(challengeMap["expire_at"]),
		Difficulty: int64From(challengeMap["difficulty"]),
	}
	answer, err := SolvePow(ctx, challenge.Challenge, challenge.Salt, challenge.ExpireAt, challenge.Difficulty)
	if err != nil {
		return "", err
	}
	return BuildPowHeader(challenge, answer)
}

func (c *Client) ListSession(ctx context.Context, token string) ([]SessionInfo, bool, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, false, errors.New("token is required")
	}
	reqURL := ListSessionURL + "?" + url.Values{"lte_cursor.pinned": {"false"}}.Encode()
	resp, status, err := c.getJSONWithStatus(ctx, reqURL, authHeaders(token))
	if err != nil {
		return nil, false, err
	}
	if status != http.StatusOK {
		return nil, false, fmt.Errorf("list session failed: status=%d body=%v", status, resp)
	}
	if code := intFrom(resp["code"]); code != 0 {
		return nil, false, fmt.Errorf("list session failed: %v", resp["msg"])
	}
	data, _ := resp["data"].(map[string]any)
	if intFrom(data["biz_code"]) != 0 {
		return nil, false, fmt.Errorf("list session failed: %v", data["biz_msg"])
	}
	bizData, _ := data["biz_data"].(map[string]any)
	sessionsRaw, _ := bizData["chat_sessions"].([]any)
	hasMore, _ := bizData["has_more"].(bool)
	sessions := make([]SessionInfo, 0, len(sessionsRaw))
	for _, raw := range sessionsRaw {
		m, _ := raw.(map[string]any)
		if m == nil {
			continue
		}
		sessions = append(sessions, SessionInfo{
			ID:        stringFromMap(m, "id"),
			Title:     stringFromMap(m, "title"),
			TitleType: stringFromMap(m, "title_type"),
			Pinned:    boolFromMap(m, "pinned"),
			UpdatedAt: floatFromMap(m, "updated_at"),
		})
	}
	return sessions, hasMore, nil
}

func (c *Client) DeleteSession(ctx context.Context, token, sessionID string) (*DeleteSessionResult, error) {
	token = strings.TrimSpace(token)
	sessionID = strings.TrimSpace(sessionID)
	if token == "" {
		return &DeleteSessionResult{SessionID: sessionID}, errors.New("token is required")
	}
	if sessionID == "" {
		return &DeleteSessionResult{SessionID: sessionID}, errors.New("session_id is required")
	}
	resp, _, err := c.postJSONWithStatus(ctx, DeleteSessionURL, authHeaders(token), NewDeleteSessionPayload(sessionID))
	if err != nil {
		return &DeleteSessionResult{SessionID: sessionID}, err
	}
	if code := intFrom(resp["code"]); code != 0 {
		return &DeleteSessionResult{SessionID: sessionID}, fmt.Errorf("delete session failed: %v", resp["msg"])
	}
	result := &DeleteSessionResult{SessionID: sessionID, Success: true}
	return result, nil
}

func (c *Client) postJSON(ctx context.Context, url string, headers map[string]string, payload any) (map[string]any, error) {
	body, _, err := c.postJSONWithStatus(ctx, url, headers, payload)
	return body, err
}

func (c *Client) getJSONWithStatus(ctx context.Context, url string, headers map[string]string) (map[string]any, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.regular.Do(req)
	if err != nil {
		req2, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if reqErr != nil {
			return nil, 0, reqErr
		}
		for k, v := range headers {
			req2.Header.Set(k, v)
		}
		resp, err = c.fallback.Do(req2)
	}
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	payloadBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	out := map[string]any{}
	if len(payloadBytes) > 0 {
		if err := json.Unmarshal(payloadBytes, &out); err != nil {
			return nil, resp.StatusCode, err
		}
	}
	return out, resp.StatusCode, nil
}

func (c *Client) streamPost(ctx context.Context, url string, headers map[string]string, payload any, powResp string) (*http.Response, error) {
	reqBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if strings.TrimSpace(powResp) != "" {
		req.Header.Set("x-ds-pow-response", powResp)
	}
	resp, err := c.stream.Do(req)
	if err != nil {
		req2, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
		if reqErr != nil {
			return nil, reqErr
		}
		for k, v := range headers {
			req2.Header.Set(k, v)
		}
		if strings.TrimSpace(powResp) != "" {
			req2.Header.Set("x-ds-pow-response", powResp)
		}
		return c.fallbackS.Do(req2)
	}
	return resp, nil
}

func (c *Client) postJSONWithStatus(ctx context.Context, url string, headers map[string]string, payload any) (map[string]any, int, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	for k, v := range jsonHeaders(headers) {
		req.Header.Set(k, v)
	}
	resp, err := c.regular.Do(req)
	if err != nil {
		req2, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if reqErr != nil {
			return nil, 0, reqErr
		}
		for k, v := range jsonHeaders(headers) {
			req2.Header.Set(k, v)
		}
		resp, err = c.fallback.Do(req2)
	}
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	payloadBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	out := map[string]any{}
	if len(payloadBytes) > 0 {
		if err := json.Unmarshal(payloadBytes, &out); err != nil {
			return nil, resp.StatusCode, err
		}
	}
	return out, resp.StatusCode, nil
}
