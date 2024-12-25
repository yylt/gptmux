package deepseekapi

import (
	"context"
	"fmt"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/yylt/gptmux/mux"
	"github.com/yylt/gptmux/pkg/util"
	"k8s.io/klog/v2"
)

type Conf struct {
	// chat
	Apikey string `yaml:"apikey"`
	Debug  bool   `yaml:"debug,omitempty"`
	Index  int    `yaml:"index,omitempty"`
}

type Dseek struct {
	c *Conf

	*openai.LLM
}

func New(c *Conf) *Dseek {
	if c == nil || c.Apikey == "" {
		klog.Warningf("deepseek api config is invalid: %v", c)
		return nil
	}

	llm, err := openai.New(
		openai.WithBaseURL("https://api.deepseek.com"),
		openai.WithToken(c.Apikey),
		openai.WithModel("deepseek-chat"),
		openai.WithHTTPClient(util.NewDebugHTTPClient("", c.Debug)),
	)
	if err != nil {
		klog.Errorf("deepseek api err: %v", err)
		return nil
	}
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
	if msg == nil {
		return nil, fmt.Errorf("no messages")
	}
	var (
		opt          = &llms.CallOptions{}
		bctx, cancle = context.WithCancel(ctx)
	)
	for _, o := range options {
		o(opt)
	}

	defer func() {
		cancle()
		if opt.StreamingFunc != nil {
			opt.StreamingFunc(bctx, nil)
		}
	}()

	return d.LLM.GenerateContent(ctx, msg, options...)

}