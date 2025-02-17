package merlin

import (
	"github.com/google/uuid"
)

type eventType string

const (
	chunk  eventType = "CHUNK"
	system eventType = "SYSTEM"
	done   eventType = "DONE"

	usageMerlin string = "merlin"
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

type useage struct {
	Used  int `json:"usage"`
	Limit int `json:"limit"`
}
type EventResp struct {
	Status string     `json:"status"`
	Data   *eventData `json:"data"`
}

type UserData struct {
	Feature struct {
		Merlin useage `json:"merlin"`
	} `json:"features,omitempty"`
}

type eventData struct {
	Content string `json:"content,omitempty"`
	Type    string `json:"eventType"`
	Attachs []struct {
		Id   string `json:"id,omitempty"`
		Type string `json:"type,omitempty"`
		Url  string `json:"url"`
	} `json:"attachments,omitempty"`
	Usage struct {
		Used  int `json:"used"`
		Limit int `json:"limit"`
	} `json:"usage,omitempty"`
}

type TokenResp struct {
	Status string `json:"status"`
	Data   struct {
		Access  string `json:"accessToken"`
		Refresh string `json:"refreshToken"`
	} `json:"data"`
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
