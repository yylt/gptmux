package ollama

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"github.com/ollama/ollama/api"
	"github.com/tmc/langchaingo/llms"
	"github.com/yylt/gptmux/mux"
	"github.com/yylt/gptmux/pkg"
	"k8s.io/klog/v2"
)

const maxBufferSize = 512 * 1024

var (
	headers = map[string]string{
		"accept":       "*/*",
		"content-type": "application/json",
	}
	defaultClient = &http.Client{
		Transport: http.DefaultTransport,
	}
)

type Config struct {
	Model  string `yaml:"model_name"`
	Server string `yaml:"server"`
	Index  int    `yaml:"index,omitempty"`
}
type ollamaResp struct {
	Resp string `yaml:"response,omitempty"`
	Done bool   `yaml:"done,omitempty"`
}

type ollm struct {
	ctx context.Context

	mu sync.RWMutex

	index int

	c *Config

	cli *api.Client
}

func New(ctx context.Context, cfg *Config) *ollm {
	if cfg == nil || cfg.Server == "" || cfg.Model == "" {
		klog.Warningf("ollama config is invalid: %v", cfg)
		return nil
	}
	u, err := url.Parse(cfg.Server)
	if err != nil {
		klog.Errorf("ollama server failed: %s", err)
		return nil
	}

	return &ollm{
		ctx:   ctx,
		c:     cfg,
		index: cfg.Index,
		cli:   api.NewClient(u, defaultClient),
	}
}

func (d *ollm) Name() string {
	return "ollama"
}

func (d *ollm) Index() int {
	return d.index
}

func (d *ollm) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	if !d.mu.TryLock() {
		return nil, pkg.BusyErr
	}
	defer d.mu.Unlock()
	prompt, model := mux.GeneraPrompt(messages)

	if model != mux.TxtModel {
		return nil, fmt.Errorf("not support model '%s'", model)
	}
	var (
		opt          = &llms.CallOptions{}
		bctx, cancle = context.WithCancel(ctx)
		data         = &llms.ContentResponse{}

		once sync.Once
	)

	for _, o := range options {
		o(opt)
	}

	d.cli.Generate(bctx, &api.GenerateRequest{
		Model:  d.c.Model,
		Prompt: prompt,
	}, func(gr api.GenerateResponse) error {
		data.Choices = append(data.Choices, &llms.ContentChoice{
			Content: gr.Response,
		})
		if gr.Done {
			data.Choices = append(data.Choices, &llms.ContentChoice{
				StopReason: "stop",
			})
			once.Do(cancle)
		}
		if opt.StreamingFunc != nil {
			err := opt.StreamingFunc(bctx, []byte(gr.Response))
			if err != nil {
				once.Do(cancle)
				return err
			}
		}
		return nil
	})

	return data, nil
}

func (d *ollm) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	return "", fmt.Errorf("not implement")
}
