package serve

type AnthropicMessageRequest struct {
	Model  string `json:"model,omitempty"`
	Stream bool   `json:"stream,omitempty"`
}

type AnthropicMessageResponse struct {
	Type  string `json:"type,omitempty"`
	Error string `json:"error,omitempty"`
}
