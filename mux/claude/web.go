package claude

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	tlsclient "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	"github.com/gofrs/uuid/v5"
	"github.com/tmc/langchaingo/llms"
	"github.com/yylt/gptmux/mux"
	"github.com/yylt/gptmux/pkg"
	"github.com/yylt/gptmux/pkg/util"
	"k8s.io/klog/v2"
)

var (
	lastupdate time.Time

	ClaudeName   = "claude"
	ClaudeChatid = "claudeid"

	headers = map[string]string{
		"Origin":          "https://claude.ai",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8,zh-Hans;q=0.7",
		"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	}
)

type Conf struct {
	ChatUuid   string `yaml:"chat_id,omitempty"`
	OrgId      string `yaml:"org_id,omitempty"`
	SessionKey string `yaml:"session_key,omitempty"`
	Index      int    `yaml:"index,omitempty"`
}

type web struct {
	ctx context.Context
	c   *Conf

	mu sync.RWMutex

	tlscli tlsclient.HttpClient
}

func New(ctx context.Context, cf *Conf) *web {
	if cf == nil || cf.ChatUuid == "" || cf.OrgId == "" || cf.SessionKey == "" {
		klog.Warningf("claude config is invalid: %v", cf)
		return nil
	}
	s := &web{
		c:   cf,
		ctx: ctx,
	}
	var (
		opts = []tlsclient.HttpClientOption{
			tlsclient.WithRandomTLSExtensionOrder(), // Chrome 107+
			tlsclient.WithClientProfile(profiles.Chrome_120),
		}
	)
	if p := util.GetEnvAny("HTTP_PROXY", "http_proxy"); p != "" {
		opts = append(opts, tlsclient.WithProxyUrl(p))
	}
	// Reference: https://bogdanfinn.gitbook.io/open-source-oasis/tls-client/client-options
	tr, err := util.NewRoundTripper(opts...)
	if err != nil {
		panic(err)
	}
	s.tlscli = tr.Client

	return s
}

// 排序
func (c *web) Index() int {
	return c.c.Index
}

func (c *web) Name() string {
	return ClaudeName
}

func (c *web) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	if !c.mu.TryLock() {
		return nil, fmt.Errorf("pending")
	}
	defer c.mu.Unlock()
	prompt, model := mux.GeneraPrompt(messages)

	if model != mux.TxtModel {
		return nil, fmt.Errorf("not support model '%s'", model)
	}
	var (
		opt          = &llms.CallOptions{}
		bctx, cancle = context.WithCancel(ctx)
		data         = &llms.ContentResponse{}
	)
	for _, o := range options {
		o(opt)
	}
	defer cancle()
	resp, err := c.chat(prompt)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("chat claude failed: %s, code: %v", http.StatusText(resp.StatusCode), resp.StatusCode)
	}
	process(resp.Body, func(er *eventResp) error {
		content, done := text(er)
		data.Choices = append(data.Choices, &llms.ContentChoice{
			Content: content,
		})
		if done {
			data.Choices = append(data.Choices, &llms.ContentChoice{
				StopReason: "stop",
			})
		}
		if opt.StreamingFunc != nil {
			return opt.StreamingFunc(bctx, []byte(content))
		}
		return nil
	})
	return data, nil
}

func (c *web) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	return "", fmt.Errorf("not implement")
}

func (c *web) chat(prompt string) (*fhttp.Response, error) {
	address := fmt.Sprintf("https://claude.ai/api/organizations/%s/chat_conversations/%s/completion", c.c.OrgId, c.c.ChatUuid)

	bs, err := json.Marshal(map[string]any{
		"prompt":         prompt,
		"rendering_mode": "messages",
		"timezone":       "Asia/Shanghai",
	})
	if err != nil {
		return nil, err
	}
	req, err := fhttp.NewRequest(http.MethodPost, address, bytes.NewReader(bs))
	if err != nil {
		return nil, err
	}

	req.AddCookie(&fhttp.Cookie{
		Name:  "sessionKey",
		Value: c.c.SessionKey,
	})

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("content-type", "application/json")
	for k, v := range headers {
		req.Header.Add(k, v)
	}

	resp, err := c.tlscli.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *web) Send(prompt string, t mux.ChatModel) (<-chan *pkg.BackResp, error) {
	resp, err := c.chat(prompt)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		resp.Body.Close()
		// newchat
		return nil, fmt.Errorf("chat claude failed: %s, code: %v", http.StatusText(resp.StatusCode), resp.StatusCode)
	}

	var rsch = make(chan *pkg.BackResp, 16)
	go func(sch chan *pkg.BackResp, body io.ReadCloser) {
		var (
			respData = &eventResp{}
			err      error
		)
		defer body.Close()
		scanner := bufio.NewScanner(body)
		for scanner.Scan() {
			line := scanner.Bytes()

			if !bytes.HasPrefix(line, util.HeaderData) {
				continue
			}
			err = json.Unmarshal(bytes.TrimPrefix(line, util.HeaderData), &respData)
			if err != nil {
				klog.Warningf("invalid response data struct: %v", err)
				continue
			}
			evresp := textProcess(respData)
			if evresp != nil {
				sch <- evresp
			}
		}
		sch <- &pkg.BackResp{
			Err: scanner.Err(),
		}
		close(sch)
	}(rsch, resp.Body)
	return rsch, nil
}

func (c *web) newchat() (string, error) {
	id, err := uuid.NewV4()
	if err != nil {
		return "", err
	}

	payload := map[string]interface{}{
		"uuid": id.String(),
		"name": "",
	}
	url := fmt.Sprintf("https://claude.ai/api/organizations/%s/chat_conversations", c.c.OrgId)
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := fhttp.NewRequest(http.MethodPost, url, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", err
	}

	req.AddCookie(&fhttp.Cookie{
		Name:  "sessionKey",
		Value: c.c.SessionKey,
	})

	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("content-type", "application/json")
	for k, v := range headers {
		req.Header.Add(k, v)
	}

	resp, err := c.tlscli.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("newchat failed: %d, body: %v", resp.StatusCode, resp.Body)
	}
	return id.String(), nil
}
