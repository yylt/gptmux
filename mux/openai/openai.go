package openai

import (
	"context"
	"fmt"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/yylt/gptmux/mux"
	"github.com/yylt/gptmux/pkg/util"
	"k8s.io/klog/v2"
)

const (
	name = "openai"
)

type Conf struct {
	Name string `yaml:"name,omitempty"`
	// https://api.deepseek.com + deepseek-chat
	// https://api.siliconflow.com + Qwen/Qwen2.5-Coder-7B-Instruct
	Baseurl string `yaml:"baseurl"`
	Apikey  string `yaml:"apikey"`
	Model   string `yaml:"model"`
	Debug   bool   `yaml:"debug,omitempty"`
	Index   int    `yaml:"index,omitempty"`
}

func (c *Conf) valid() error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}
	if c.Apikey == "" {
		return fmt.Errorf("apikey is empty")
	}
	if c.Model == "" {
		return fmt.Errorf("model is empty")
	}
	return nil
}

type Openai struct {
	c *Conf

	*openai.LLM
}

func New(ctx context.Context, c *Conf) *Openai {
	if err := c.valid(); err != nil {
		klog.Infof("openai config is invalid: %v", c)
		return nil
	}
	if c.Name == "" {
		c.Name = name
	}

	llm, err := openai.New(
		openai.WithBaseURL(c.Baseurl),
		openai.WithToken(c.Apikey),
		openai.WithModel(c.Model),
		openai.WithHTTPClient(util.NewDebugHTTPClient("", c.Debug)),
	)

	if err != nil {
		klog.Errorf("silicon api err: %v", err)
		return nil
	}

	slicon := &Openai{
		c:   c,
		LLM: llm,
	}
	return slicon
}

func (d *Openai) Name() string {
	return d.c.Name
}

func (d *Openai) Index() int {
	return d.c.Index
}

func (d *Openai) Completion(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (string, error) {
	return "", fmt.Errorf("not found message")
}

func (d *Openai) GenerateContent(ctx context.Context, messages []llms.MessageContent,
	options ...llms.CallOption) (resp *llms.ContentResponse, err error) {
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
		if err == nil && opt.StreamingFunc != nil {
			opt.StreamingFunc(bctx, nil)
		}
	}()

	resp, err = d.LLM.GenerateContent(ctx, msg, options...)
	return
}
