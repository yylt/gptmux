package merlin

import (
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

type EventResp struct {
	Status string     `json:"status"`
	Data   *eventData `json:"data"`
}

type UserData struct {
	User struct {
		Used  int    `json:"used"`
		Type  string `json:"type"`
		Limit int    `json:"limit"`
	} `json:"user"`
}

type eventData struct {
	Content string `json:"content,omitempty"`
	Attachs []struct {
		Id   string `json:"id,omitempty"`
		Type string `json:"type,omitempty"`
		Url  string `json:"url"`
	} `json:"attachments,omitempty"`
	Type  string `json:"eventType"`
	Usage struct {
		Used  int    `json:"used"`
		Type  string `json:"type"`
		Limit int    `json:"limit"`
	} `json:"usage,omitempty"`
	Setting struct {
		Id string `json:"chatId,omitempty"`
		Ts string `json:"timestamp,omitempty"`
	} `json:"settings,omitempty"`
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
