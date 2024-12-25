package silicon

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
	Model  string `yaml:"model"`
	Debug  bool   `yaml:"debug,omitempty"`
	Index  int    `yaml:"index,omitempty"`
}

type Sli struct {
	c *Conf

	*openai.LLM
}

func New(ctx context.Context, c *Conf) *Sli {
	if c == nil || c.Apikey == "" {
		klog.Infof("silicon config is invalid: %v", c)
		return nil
	}

	llm, err := openai.New(
		openai.WithBaseURL("https://api.siliconflow.com"),
		openai.WithToken(c.Apikey),
		openai.WithModel(c.Model),
		openai.WithHTTPClient(util.NewDebugHTTPClient("", c.Debug)),
	)

	if err != nil {
		klog.Errorf("silicon api err: %v", err)
		return nil
	}

	slicon := &Sli{
		c:   c,
		LLM: llm,
	}
	return slicon
}

func (d *Sli) Name() string {
	return "silicon-api"
}

func (d *Sli) Index() int {
	return d.c.Index
}

func (d *Sli) Completion(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (string, error) {
	return "", fmt.Errorf("not found message")
}

func (d *Sli) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	msg := mux.NormalPrompt(messages)
	if msg == nil {
		return nil, fmt.Errorf("not found message")
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
