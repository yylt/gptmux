package ds

import (
	"maps"
	"strings"
)

const (
	DeepSeekHost                 = "chat.deepseek.com"
	LoginURL                     = "https://chat.deepseek.com/api/v0/users/login"
	CreateSessionURL             = "https://chat.deepseek.com/api/v0/chat_session/create"
	CreatePowURL                 = "https://chat.deepseek.com/api/v0/chat/create_pow_challenge"
	CompletionURL                = "https://chat.deepseek.com/api/v0/chat/completion"
	ListSessionURL               = "https://chat.deepseek.com/api/v0/chat_session/fetch_page"
	DeleteSessionURL             = "https://chat.deepseek.com/api/v0/chat_session/delete"
	CompletionTargetPath         = "/api/v0/chat/completion"
	DeepSeekClientName           = "DeepSeek"
	DeepSeekClientPlatform       = "android"
	DeepSeekClientVersion        = "2.0.1"
	DeepSeekClientAndroidVersion = "35"
	DeepSeekClientLocale         = "zh_CN"
)

var baseHeaders = map[string]string{
	"Host":              DeepSeekHost,
	"Accept":            "application/json",
	"Content-Type":      "application/json",
	"accept-charset":    "UTF-8",
	"User-Agent":        DeepSeekClientName + "/" + DeepSeekClientVersion + " Android/" + DeepSeekClientAndroidVersion,
	"x-client-platform": DeepSeekClientPlatform,
	"x-client-version":  DeepSeekClientVersion,
	"x-client-locale":   DeepSeekClientLocale,
}

func authHeaders(token string) map[string]string {
	headers := make(map[string]string, len(baseHeaders)+1)
	maps.Copy(headers, baseHeaders)
	headers["authorization"] = "Bearer " + strings.TrimSpace(token)
	return headers
}

func jsonHeaders(headers map[string]string) map[string]string {
	out := make(map[string]string, len(headers)+1)
	for k, v := range headers {
		out[k] = v
	}
	out["Content-Type"] = "application/json"
	return out
}

func completionRequestBody(payload CompletionPayload) map[string]any {
	body := map[string]any{
		"chat_session_id":   payload.ChatSessionID,
		"model_type":        payload.ModelType,
		"parent_message_id": payload.ParentMessageID,
		"prompt":            payload.Prompt,
		"ref_file_ids":      payload.RefFileIDs,
		"thinking_enabled":  payload.ThinkingEnabled,
		"search_enabled":    payload.SearchEnabled,
	}
	for k, v := range payload.PassThrough {
		body[k] = v
	}
	return body
}

func sessionIDFromBizData(bizData map[string]any) string {
	if bizData == nil {
		return ""
	}
	if sessionID, _ := bizData["id"].(string); strings.TrimSpace(sessionID) != "" {
		return strings.TrimSpace(sessionID)
	}
	if chatSession, _ := bizData["chat_session"].(map[string]any); chatSession != nil {
		if sessionID, _ := chatSession["id"].(string); strings.TrimSpace(sessionID) != "" {
			return strings.TrimSpace(sessionID)
		}
	}
	return ""
}

func stringFromMap(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func boolFromMap(m map[string]any, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

func floatFromMap(m map[string]any, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return 0
}

func int64From(v any) int64 {
	switch n := v.(type) {
	case int:
		return int64(n)
	case int8:
		return int64(n)
	case int16:
		return int64(n)
	case int32:
		return int64(n)
	case int64:
		return n
	case float32:
		return int64(n)
	case float64:
		return int64(n)
	default:
		return 0
	}
}

func intFrom(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int8:
		return int(n)
	case int16:
		return int(n)
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float32:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}
