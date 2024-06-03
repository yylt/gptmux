package ollama

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/yylt/gptmux/pkg"
)

type Config struct {
	Model  string `yaml:"model_name"`
	Server string `yaml:"server"`
	Index  int    `yaml:"index,omitempty"`
}

type ollm struct {
	mu sync.RWMutex

	index int

	llm *ollama.LLM
}

func New(cfg *Config) *ollm {
	if cfg == nil {
		return nil
	}
	var (
		opts []ollama.Option
	)
	opts = append(opts, ollama.WithModel(cfg.Model), ollama.WithServerURL(cfg.Server))

	u, err := url.Parse(cfg.Server)
	if err != nil {
		panic(err)
	}

	if u.Scheme == "https" {
		opts = append(opts, ollama.WithHTTPClient(
			&http.Client{Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}},
		))
	}
	llm, err := ollama.New(opts...)
	if err != nil {
		panic(err)
	}
	return &ollm{
		llm:   llm,
		index: cfg.Index,
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
	return d.llm.GenerateContent(ctx, messages, options...)
}

func (d *ollm) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	return "", fmt.Errorf("not implement")
}
