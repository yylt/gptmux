package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"

	api "github.com/yylt/gptmux/api/go"
	"github.com/yylt/gptmux/mux"
	"github.com/yylt/gptmux/pkg"
	"github.com/yylt/gptmux/pkg/util"
	"k8s.io/klog/v2"
)

const (
	name = "openai"
)

var (
	HeaderDefault = map[string]string{
		"accept-language": "zh-CN,zh;q=0.9,en;q=0.8,zh-Hans;q=0.7",
		"Content-Type":    "application/json",
	}
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

	aa  *openai.LLM
	cli *http.Client
}

func New(ctx context.Context, c *Conf) *Openai {
	if err := c.valid(); err != nil {
		klog.Infof("openai config is invalid: %v", c)
		return nil
	}
	if c.Name == "" {
		c.Name = name
	}

	slicon := &Openai{
		c:   c,
		cli: util.NewDebugHTTPClient("", c.Debug),
	}
	return slicon
}

func (d *Openai) Name() string {
	return d.c.Name
}

func (d *Openai) Index() int {
	return d.c.Index
}

func (d *Openai) Completion(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	var (
		opt          = &llms.CallOptions{}
		bctx, cancle = context.WithCancel(ctx)
		err          error
	)
	for _, o := range options {
		o(opt)
	}
	defer cancle()
	var (
		req    = opt.Metadata[mux.ReqBody].(*api.V1CompletionsPostRequest)
		newreq = new(api.V1ChatCompletionsPostRequest)
	)

	pkg.Trans(req, newreq)
	newreq.Model = d.c.Model
	newreq.Stream = true
	newreq.Messages = []api.V1ChatCompletionsPostRequestMessagesInner{
		{
			Role:    mux.RoleUser,
			Content: prompt,
		},
	}
	bs, err := json.Marshal(newreq)
	if err != nil {
		return "", err
	}
	resp, err := d.chat(d.c.Baseurl+"/v1/chat/completions", bs)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var (
		buf      = util.GetBuf()
		respData api.V1CompletionsPost200Response
	)

	defer util.PutBuf(buf)

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if !bytes.HasPrefix(line, util.HeaderData) {
			continue
		}
		err = json.Unmarshal(bytes.TrimPrefix(line, util.HeaderData), &respData)
		if err != nil {
			klog.Warningf("parse event data failed: %v", err)
			continue
		}
		for _, choci := range respData.Choices {
			if choci.Text == "" {
				continue
			}
			if opt.StreamingFunc != nil {
				err = opt.StreamingFunc(bctx, []byte(choci.Text))
				if err != nil {
					break
				}
			}
			buf.WriteString(choci.Text)
		}
	}
	return buf.String(), nil
}

func (d *Openai) GenerateContent(ctx context.Context, messages []llms.MessageContent,
	options ...llms.CallOption) (*llms.ContentResponse, error) {

	var (
		opt          = &llms.CallOptions{}
		bctx, cancle = context.WithCancel(ctx)
		err          error
	)
	for _, o := range options {
		o(opt)
	}
	defer cancle()
	req := opt.Metadata[mux.ReqBody].(*api.V1ChatCompletionsPostRequest)
	req.Model = d.c.Model
	req.Stream = true

	bs, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	resp, err := d.chat(d.c.Baseurl+"/v1/chat/completions", bs)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var (
		buf      = util.GetBuf()
		respData api.V1ChatCompletionsPost200Response
		ret      = new(llms.ContentResponse)
	)

	defer util.PutBuf(buf)

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			cancle()
			if opt.StreamingFunc != nil {
				opt.StreamingFunc(bctx, nil)
			}
			return ret, io.EOF
		default:
		}
		line := scanner.Bytes()
		if !bytes.HasPrefix(line, util.HeaderData) {
			continue
		}
		err = json.Unmarshal(bytes.TrimPrefix(line, util.HeaderData), &respData)
		if err != nil {
			klog.Warningf("parse event data failed: %v", err)
			continue
		}
		for _, choci := range respData.Choices {
			ret.Choices = append(ret.Choices, &llms.ContentChoice{
				Content:    choci.Delta.Content,
				StopReason: choci.FinishReason,
			})
			if choci.Delta.Content == "" {
				continue
			}
			if opt.StreamingFunc != nil {
				err = opt.StreamingFunc(bctx, []byte(choci.Delta.Content))
				if err != nil {
					break
				}
			}
			buf.WriteString(choci.Delta.Content)
		}
	}
	cancle()
	opt.StreamingFunc(bctx, nil)
	return ret, nil
}
func (d *Openai) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	return "", fmt.Errorf("not implement")
}
func (d *Openai) chat(addr string, body []byte) (*http.Response, error) {
	var buf = &bytes.Buffer{}
	if body != nil {
		buf = bytes.NewBuffer(body)
	}
	req, err := http.NewRequest(http.MethodPost, addr, buf)
	if err != nil {
		return nil, err
	}
	for k, v := range HeaderDefault {
		req.Header.Set(k, v)
	}
	req.Header.Set("Authorization", "Bearer "+d.c.Apikey)

	resp, err := d.cli.Do(req)
	if err != nil {
		return nil, err
	}
	if !util.IsHttp20xCode(resp.StatusCode) {
		resp.Body.Close()
		return nil, fmt.Errorf("request '%s' failed: %v, code: %d", addr, http.StatusText(resp.StatusCode), resp.StatusCode)
	}
	return resp, nil
}
