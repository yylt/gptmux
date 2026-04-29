package response

type OpenAIChatMessage struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

type OpenAIChatChoice struct {
	Delta        *OpenAIChatMessage `json:"delta,omitempty"`
	FinishReason string             `json:"finish_reason,omitempty"`
}

type OpenAIModel struct {
	ID     string `json:"id"`
	Model  string `json:"model"`
	Object string `json:"object"`
}

type OpenAIChatResponse struct {
	ID      string              `json:"id,omitempty"`
	Object  string              `json:"object,omitempty"`
	Model   string              `json:"model,omitempty"`
	Choices []*OpenAIChatChoice `json:"choices,omitempty"`
}

type OpenAIChatRequest struct {
	Messages        []*OpenAIChatMessage `json:"messages"`
	Model           string               `json:"model,omitempty"`
	Stream          bool                 `json:"stream,omitempty"`
	Temperature     float32              `json:"temperature,omitempty"`
	PresencePenalty int                  `json:"presence_penalty,omitempty"`
}

func NewOpenAIChatResponse(req *OpenAIChatRequest, finish bool, content string) *OpenAIChatResponse {
	resp := &OpenAIChatResponse{
		Model:  req.Model,
		ID:     "chatcmpl",
		Object: "chat.completion",
	}
	if finish {
		resp.Choices = append(resp.Choices, &OpenAIChatChoice{
			FinishReason: content,
		})
		return resp
	}
	resp.Choices = append(resp.Choices, &OpenAIChatChoice{
		Delta: &OpenAIChatMessage{
			Role:    "assistant",
			Content: content,
		},
	})
	return resp
}
