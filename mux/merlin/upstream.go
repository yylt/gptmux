package merlin

import (
	"fmt"
	"path"

	"github.com/google/uuid"
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

type UserResp struct {
	Status string    `json:"status"`
	Data   *UserData `json:"data"`
}

type UserData struct {
	User *Usage `json:"user"`
}

type Usage struct {
	Used  int    `json:"used"`
	Type  string `json:"type"`
	Limit int    `json:"limit"`
}

type EventResp struct {
	Status string     `json:"status"`
	Data   *eventData `json:"data"`
}

type Attachment struct {
	Id   string `json:"id,omitempty"`
	Type string `json:"type,omitempty"`
	Url  string `json:"url"`
}

type setting struct {
	Id string `json:"chatId,omitempty"`
	Ts string `json:"timestamp,omitempty"`
}
type eventData struct {
	Content string        `json:"content,omitempty"`
	Attachs []*Attachment `json:"attachments,omitempty"`
	Type    string        `json:"eventType"`
	Usage   *Usage        `json:"usage,omitempty"`
	Setting *setting      `json:"settings,omitempty"`
}

type TokenResp struct {
	Status string     `json:"status"`
	Data   *tokenData `json:"data"`
}
type tokenData struct {
	Access  string `json:"accessToken"`
	Refresh string `json:"refreshToken"`
}

func getStatusUrl(merlinbase string, token string) string {
	surl := path.Join(merlinbase, fmt.Sprintf("status?firebaseToken=%s&from=DASHBOARD", token))
	return "https://" + surl
}

func getAuth1Url(authbase string, key string) string {
	surl := path.Join(authbase, fmt.Sprintf("/v1/accounts:signInWithPassword?key=%s", key))
	return "https://" + surl
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
	surl := path.Join(merlinbase, "session/get")
	return "https://" + surl
}

func getAuth2Body(idtoken string) map[string]interface{} {
	return map[string]interface{}{
		"token": idtoken,
	}
}

func getChatUrl(merlinbase string) string {
	surl := path.Join(merlinbase, "thread?customJWT=true&version=1.1")
	return "https://" + surl
}

func getImageUrl(merlinbase string) string {
	surl := path.Join(merlinbase, "thread/image-generation?customJWT=true")
	return "https://" + surl
}

func chatBody(prompt string, model string) map[string]interface{} {
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
		"chatId": uuid.New().String(),
		"mode":   "VANILLA_CHAT",
		"model":  model,
	}
}

func imageBody(prompt string, model string) map[string]interface{} {
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
		"metadata": map[string]interface{}{
			"aspectRatio": "1:1",
			"numImages":   1,
		},
		"chatId": uuid.New().String(),
		"mode":   "IMAGE_CHAT",
		"model":  model,
	}
}
