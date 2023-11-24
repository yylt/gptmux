package pkg

const (
	GPT3Model     = "gpt-3.5-turbo"
	GPT3PlusModel = "gpt-35-turbo"
	GPT4Model     = "gpt-4"
	GPT4PlusModel = "gpt-4-32k"

	AssistantRole = "assistant"
)

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
				Role:    AssistantRole,
			},
		})
	}
	return resp
}

func GetModel(t PromptType) *Model {
	switch t {
	case Code:
		return &Model{
			Id:     GPT3Model,
			Model:  GPT3Model,
			Object: "model",
		}
	case TextGpt3:
		return &Model{
			Id:     GPT3PlusModel,
			Model:  GPT3PlusModel,
			Object: "model",
		}
	case TextGpt4:
		return &Model{
			Id:     GPT4Model,
			Model:  GPT4Model,
			Object: "model",
		}
	default:
		return nil
	}
}
