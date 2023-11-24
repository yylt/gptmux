package pkg

type PromptType int

const (
	TextGpt3 PromptType = iota
	TextGpt4
	Code
	Image
)

type Backender interface {
	// block read
	Send(prompt string, t PromptType) (<-chan *BackResp, error)

	// model
	Model() []PromptType
}

type BackResp struct {
	Err     error
	Content string
}
