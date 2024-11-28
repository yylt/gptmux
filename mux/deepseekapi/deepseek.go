package deepseekapi

import (
	"context"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/yylt/gptmux/mux"
	"k8s.io/klog/v2"
)

type Conf struct {
	// chat
	Apikey string `yaml:"apikey,omitempty"`
	Index  int    `yaml:"index,omitempty"`
}

type Dseek struct {
	c *Conf

	*openai.LLM
}

func New(c *Conf) *Dseek {
	if c == nil || c.Apikey == "" {
		klog.Infof("deepseek api config is null")
		return nil
	}

	llm, err := openai.New(
		openai.WithBaseURL("https://api.deepseek.com"),
		openai.WithToken(c.Apikey),
		openai.WithModel("deepseek-chat"),
	)
	if err != nil {
		klog.Errorf("deepseek api err: %v", err)
		return nil
	}
	klog.Infof("deepseek config is: %#v", c)
	seek := &Dseek{
		c:   c,
		LLM: llm,
	}
	return seek
}

func (d *Dseek) Name() string {
	return "deepseek-api"
}

func (d *Dseek) Index() int {
	return d.c.Index
}
func (d *Dseek) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	msg := mux.NormalPrompt(messages)
	if msg != nil {
		return d.LLM.GenerateContent(ctx, msg, options...)
	}
	return nil, nil
}
