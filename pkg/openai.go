package pkg

import (
	api "github.com/yylt/gptmux/api/go"
)

// backend response
type BackResp struct {
	Err     error
	Content string
	Cookie  map[string]string
}

type Delta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

type Choice struct {
	Delta  *Delta `json:"delta,omitempty"`
	Finish string `json:"finish_reason,omitempty"`
}

// model request
type Model struct {
	Id     string `json:"id"`
	Model  string `json:"model"` // equal id
	Object string `json:"object"`
}

type ChatResp struct {
	Id      string    `json:"id,omitempty"`
	Object  string    `json:"object,omitempty"`
	Model   string    `json:"model,omitempty"`
	Choices []*Choice `json:"choices,omitempty"`
}

// gpt request
type ChatReq struct {
	Messages    []*Delta `json:"messages"`
	Model       string   `json:"model,omitempty"`
	Stream      bool     `json:"stream,omitempty"`
	Temperature float32  `json:"temperature,omitempty"`
	Presence    int      `json:"presence_penalty,omitempty"`
}

func GetContent(req *ChatReq, finish bool, content string) *ChatResp {
	resp := &ChatResp{
		Model:  req.Model,
		Id:     "chatcmpl",
		Object: "chat.completion",
	}
	if finish {
		resp.Choices = append(resp.Choices, &Choice{
			Finish: content,
		})
	} else {
		resp.Choices = append(resp.Choices, &Choice{
			Delta: &Delta{
				Content: content,
				Role:    "assistant",
			},
		})
	}
	return resp
}

func Trans(src *api.V1CompletionsPostRequest, dst *api.V1ChatCompletionsPostRequest) {
	if src == nil || dst == nil {
		return
	}
	dst.FrequencyPenalty = src.FrequencyPenalty
	dst.N = src.N
	dst.TopP = src.TopP
}
