package zhipu

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/swxctx/goai/zhipu"
	"github.com/tmc/langchaingo/llms"
	"github.com/yylt/gptmux/mux"
	"github.com/yylt/gptmux/pkg"
	"github.com/yylt/gptmux/pkg/util"
	"k8s.io/klog/v2"
)

type Conf struct {
	ApiKey string `yaml:"apikey"`
	Debug  bool   `yaml:"debug,omitempty"`
	Index  int    `yaml:"index,omitempty"`
}

type Zp struct {
	c  *Conf
	mu sync.RWMutex
}

func New(c *Conf) *Zp {
	if c == nil || c.ApiKey == "" {
		klog.Infof("zhipu config is null")
		return nil
	}
	err := zhipu.NewClient(c.ApiKey, c.Debug)
	if err != nil {
		klog.Infof("zhipu login failed: %s", err)
		return nil
	}
	return &Zp{
		c: c,
	}
}

func (d *Zp) Name() string {
	return "zhipu"
}

func (d *Zp) Index() int {
	return d.c.Index
}
func (d *Zp) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	return "", fmt.Errorf("not implement")
}

func (d *Zp) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	if !d.mu.TryLock() {
		return nil, pkg.BusyErr
	}
	defer d.mu.Unlock()
	prompt, model := mux.GeneraPrompt(messages)

	if model != mux.TxtModel {
		return nil, fmt.Errorf("not support model '%s'", model)
	}
	read, err := zhipu.ChatStream(&zhipu.ChatRequest{
		Model: "glm-4-flash",
		Messages: []zhipu.MessageInfo{
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return nil, err
	}

	var (
		respData     = &pkg.ChatResp{}
		opt          = &llms.CallOptions{}
		bctx, cancle = context.WithCancel(ctx)
		data         = &llms.ContentResponse{}
		ret          = &pkg.BackResp{}
		body         = read.Response().Body
	)
	for _, o := range options {
		o(opt)
	}
	defer cancle()

	defer body.Close()
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if !bytes.HasPrefix(line, util.HeaderData) {
			continue
		}

		err = json.Unmarshal(bytes.TrimPrefix(line, util.HeaderData), &respData)
		if err != nil {
			klog.Warningf("parse event data failed: %v, content: %s", err, string(bytes.TrimPrefix(line, util.HeaderData)))
			continue
		}
		if d.c.Debug {
			klog.Infof("data: %s", string(bytes.TrimPrefix(line, util.HeaderData)))
		}
		ret.Content = ""
		ret.Err = err

		for _, choci := range respData.Choices {
			if choci == nil {
				continue
			}
			if choci.Finish != "" {
				ret.Err = fmt.Errorf("")
			}
			if choci.Delta == nil {
				continue
			}
			ret.Content += choci.Delta.Content
		}

		data.Choices = append(data.Choices, &llms.ContentChoice{
			Content: ret.Content,
		})
		if ret.Err != nil {
			data.Choices = append(data.Choices, &llms.ContentChoice{
				StopReason: "stop",
			})
			cancle()
		}
		if opt.StreamingFunc != nil {
			err = opt.StreamingFunc(bctx, []byte(ret.Content))
			if err != nil || ret.Err != nil {
				break
			}
		}
	}

	return data, nil
}
