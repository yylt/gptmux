package response

type AnthropicMessage struct {
	ID           string                   `json:"id,omitempty"`
	Type         string                   `json:"type,omitempty"`
	Role         string                   `json:"role,omitempty"`
	Model        string                   `json:"model,omitempty"`
	Content      []*AnthropicContentBlock `json:"content,omitempty"`
	StopReason   string                   `json:"stop_reason,omitempty"`
	StopSequence string                   `json:"stop_sequence,omitempty"`
}

type AnthropicContentBlock struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

type AnthropicMessageDelta struct {
	StopReason   string `json:"stop_reason,omitempty"`
	StopSequence string `json:"stop_sequence,omitempty"`
}

type AnthropicTextDelta struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

type AnthropicMessageStartData struct {
	Type    string            `json:"type,omitempty"`
	Message *AnthropicMessage `json:"message,omitempty"`
}

type AnthropicContentBlockStartData struct {
	Type         string                  `json:"type,omitempty"`
	Index        int                     `json:"index,omitempty"`
	ContentBlock *AnthropicContentBlock  `json:"content_block,omitempty"`
}

type AnthropicContentBlockDeltaData struct {
	Type  string               `json:"type,omitempty"`
	Index int                  `json:"index,omitempty"`
	Delta *AnthropicTextDelta  `json:"delta,omitempty"`
}

type AnthropicContentBlockStopData struct {
	Type  string `json:"type,omitempty"`
	Index int    `json:"index,omitempty"`
}

type AnthropicMessageDeltaData struct {
	Type  string                  `json:"type,omitempty"`
	Delta *AnthropicMessageDelta  `json:"delta,omitempty"`
}

type AnthropicMessageStopData struct {
	Type string `json:"type,omitempty"`
}

func NewAnthropicMessageStart(model string) *AnthropicMessageStartData {
	return &AnthropicMessageStartData{
		Type: "message_start",
		Message: &AnthropicMessage{
			ID:      "msg_gptmux",
			Type:    "message",
			Role:    "assistant",
			Model:   model,
			Content: []*AnthropicContentBlock{},
		},
	}
}

func NewAnthropicContentBlockStart() *AnthropicContentBlockStartData {
	return &AnthropicContentBlockStartData{
		Type:  "content_block_start",
		Index: 0,
		ContentBlock: &AnthropicContentBlock{
			Type: "text",
			Text: "",
		},
	}
}

func NewAnthropicContentBlockDelta(content string) *AnthropicContentBlockDeltaData {
	return &AnthropicContentBlockDeltaData{
		Type:  "content_block_delta",
		Index: 0,
		Delta: &AnthropicTextDelta{
			Type: "text_delta",
			Text: content,
		},
	}
}

func NewAnthropicContentBlockStop() *AnthropicContentBlockStopData {
	return &AnthropicContentBlockStopData{
		Type:  "content_block_stop",
		Index: 0,
	}
}

func NewAnthropicMessageDelta(stopReason string) *AnthropicMessageDeltaData {
	return &AnthropicMessageDeltaData{
		Type: "message_delta",
		Delta: &AnthropicMessageDelta{
			StopReason: stopReason,
		},
	}
}

func NewAnthropicMessageStop() *AnthropicMessageStopData {
	return &AnthropicMessageStopData{Type: "message_stop"}
}
