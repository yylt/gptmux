package merlin

import (
	"fmt"
	"path"
)

type eventType string

const (
	chunk  eventType = "CHUNK"
	system eventType = "SYSTEM"
	done   eventType = "DONE"
)

type authResp struct {
	LocalId string `json:"localId"`
	IdToken string `json:"idToken"`
	Email   string `json:"email"`
}

type merelinResp struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data"`
}

type UserData struct {
	User *Usage `json:"user"`
}

type Usage struct {
	Used  int    `json:"used"`
	Type  string `json:"type"`
	Limit int    `json:"limit"`
}

type eventData struct {
	Content string `json:"content,omitempty"`
	Type    string `json:"eventType"`
	Usage   *Usage `json:"usage,omitempty"`
}

type tokenData struct {
	Access  string `json:"accessToken"`
	Refresh string `json:"refreshToken"`
}

func getStatusUrl(merlinbase string, token string) string {
	return path.Join(merlinbase, fmt.Sprintf("status?firebaseToken=%s&from=DASHBOARD", token))
}

func getAuth1Url(authbase string) string {
	return path.Join(authbase)
}

func getAuth1Body(user, pass string) map[string]interface{} {
	return map[string]interface{}{
		"returnSecureToken": true,
		"email":             user,
		"password":          pass,
		"clientType":        "CLIENT_TYPE_WEB",
	}
}

func getAuth2Url(merlinbase string) string {
	return path.Join(merlinbase, "session/get")
}

func getAuth2Body(idtoken string) map[string]interface{} {
	return map[string]interface{}{
		"token": idtoken,
	}
}

func getContentUrl(merlinbase string) string {
	return path.Join(merlinbase, "thread?customJWT=true&version=1.1")
}

func getContentBody(prompt string, model string) map[string]interface{} {
	return map[string]interface{}{
		"action": map[string]interface{}{
			"message": map[string]interface{}{
				"content": prompt,
				"metadata": map[string]interface{}{
					"context": "",
				},
				"parentId": "root",
				"role":     "user",
			},
			"type": "NEW",
		},
		"chatId":   "",
		"language": "CHINESE_SIMPLIFIED",
		"mode":     "VANILLA_CHAT",
		"model":    model,
	}
}
