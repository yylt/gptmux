package pkg

import "strings"

type ChatModel string

const (
	GPT3Model     ChatModel = "gpt-3.5-turbo"
	GPT3PlusModel ChatModel = "gpt-35-turbo"
	GPT4Model     ChatModel = "gpt-4"
	GPT4PlusModel ChatModel = "gpt-4-32k"

	ImgModel ChatModel = "image"
)

var (
	supportModels = []*Model{
		{
			Id:     string(GPT3Model),
			Model:  string(GPT3Model),
			Object: "model",
		},
		{
			Id:     string(GPT3PlusModel),
			Model:  string(GPT3PlusModel),
			Object: "model",
		},
		{
			Id:     string(GPT4Model),
			Model:  string(GPT4Model),
			Object: "model",
		},
		{
			Id:     string(GPT4PlusModel),
			Model:  string(GPT4PlusModel),
			Object: "model",
		},
	}
)

type Backender interface {
	// async read
	Send(prompt string, t ChatModel) (<-chan *BackResp, error)
}

type BackResp struct {
	Err     error
	Content string
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

func GetModels() []*Model {
	return supportModels
}

func ModelName(m string) (ChatModel, bool) {
	if strings.HasPrefix(m, "gpt-3") {
		return GPT3Model, true
	}
	if strings.HasPrefix(m, "gpt-4") {
		return GPT4Model, true
	}
	switch m {
	case string(ImgModel):
		return ImgModel, true

	}
	return "", false
}
