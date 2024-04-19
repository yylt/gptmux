package claude

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	fhttp "github.com/bogdanfinn/fhttp"
	tlsclient "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	"github.com/go-resty/resty/v2"
	"github.com/gofrs/uuid/v5"
	"github.com/tmc/langchaingo/llms"
	"github.com/yylt/gptmux/mux"
	"github.com/yylt/gptmux/pkg"
	"github.com/yylt/gptmux/pkg/box"
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
	// chat
	ChatUuid string `yaml:"chat_id,omitempty"`
	Index    int    `yaml:"index,omitempty"`
}

type web struct {
	ctx context.Context

	mu  sync.RWMutex
	hcs []*http.Cookie

	cli *resty.Client

	chatid string

	orgid string

	index int

	b box.Box

	tlscli tlsclient.HttpClient
}

func New(ctx context.Context, cf *Conf, b box.Box) *web {
	s := &web{
		chatid: cf.ChatUuid,
		index:  cf.Index,
		b:      b,
		ctx:    ctx,
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
	// Set as transport. Don't forget to set the UA!
	s.cli = resty.New().SetTransport(tr).SetHeaders(headers)

	go s.run()

	return s
}

// 排序
func (c *web) Index() int {
	return c.index
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

	if model != pkg.TxtModel {
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
	if resp.StatusCode != 200 {
		resp.Body.Close()
		// TODO newchat
		return nil, fmt.Errorf("chat claude failed: %s, code: %v", http.StatusText(resp.StatusCode), resp.StatusCode)
	}
	klog.V(2).Infof("upstream '%s', model: %s, prompt '%s'", c.Name(), model, strconv.Quote(prompt))
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
	cookie, err := c.cookie()
	if err != nil {
		return nil, err
	}

	address := fmt.Sprintf("https://claude.ai/api/organizations/%s/chat_conversations/%s/completion", c.orgid, c.chatid)

	bs, err := json.Marshal(map[string]any{
		"prompt":   prompt,
		"timezone": "Asia/Shanghai",
	})
	if err != nil {
		return nil, err
	}
	req, err := fhttp.NewRequest(http.MethodPost, address, bytes.NewReader(bs))
	if err != nil {
		return nil, err
	}
	for _, ck := range cookie {
		req.AddCookie(&fhttp.Cookie{
			Name:  ck.Name,
			Value: ck.Value,
		})
	}
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

func (c *web) Send(prompt string, t pkg.ChatModel) (<-chan *pkg.BackResp, error) {
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

func (c *web) newchat(cookie []*http.Cookie) (string, error) {

	id, err := uuid.NewV4()
	if err != nil {
		return "", err
	}
	klog.Infof("new chat id %s", id)
	payload := map[string]interface{}{
		"uuid": id.String(),
		"name": "",
	}
	url := fmt.Sprintf("https://claude.ai/api/organizations/%s/chat_conversations", c.orgid)
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		klog.Infof("Error marshaling payload:", err)
		return "", err
	}
	req, err := fhttp.NewRequest(http.MethodPost, url, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", err
	}
	for _, ck := range cookie {
		req.AddCookie(&fhttp.Cookie{
			Name:  ck.Name,
			Value: ck.Value,
		})
	}

	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("content-type", "application/json")
	for k, v := range headers {
		req.Header.Add(k, v)
	}

	resp, err := c.tlscli.Do(req)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		resp.Body.Close()
		return "", fmt.Errorf("newchat claude failed: %d, body: %v", resp.StatusCode)
	}
	return id.String(), nil
}

func (c *web) user(hc []*http.Cookie) (org string, err error) {
	if hc == nil {
		return "", fmt.Errorf("cookie must set")
	}
	var u = &user{}
	url := "https://claude.ai/api/auth/current_account"
	resp, err := c.cli.R().SetCookies(hc).SetResult(u).Get(url)
	klog.Infof("claude user api, response: %v, errmsg: %v", u, err)
	if err != nil {
		return "", err
	}

	defer resp.RawBody().Close()
	if util.IsHttp20xCode(resp.StatusCode()) {
		for _, m := range u.Account.Members {
			if m.Org.Name == u.Account.Dname {
				org = m.Org.Uuid
				break
			}
		}
		return org, nil
	}
	return "", fmt.Errorf("failed user api, code: %d, body: %#v", resp.StatusCode(), resp.Body())
}

// 从box获取 token，并验证是否可用
// 不可用则通知
func (c *web) cookie() ([]*http.Cookie, error) {
	if c.hcs != nil {
		_, err := c.user(c.hcs)
		if err == nil {
			return c.hcs, nil
		}
	}

	var (
		now          = time.Now()
		cookiev, idv string
	)

	err := c.b.Receive(func(m *box.Message) bool {
		t := strings.ToLower(m.Title)
		switch t {
		case ClaudeChatid:
			if idv == "" {
				idv = strings.Map(func(r rune) rune {
					if unicode.IsSpace(r) {
						return rune(0)
					}
					return r
				}, m.Msg)

				klog.Infof("title: '%v', var: '%s'", ClaudeChatid, idv)
			}

		case ClaudeName:
			if cookiev == "" {
				cookiev = strings.Map(func(r rune) rune {
					if unicode.IsSpace(r) {
						return rune(0)
					}
					return r
				}, m.Msg)

				klog.Infof("title: '%v', var: '%s'", ClaudeName, cookiev)
			}
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	// not found
	if cookiev == "" {
		if lastupdate.Add(interval).Before(now) {
			c.b.Push(&box.Message{
				Title: "require token: " + ClaudeName,
				Msg:   "更新token",
			})
			lastupdate = now
		}
		return nil, fmt.Errorf("not found claude token")
	}
	hcs := []*http.Cookie{
		{
			Name:  cookiekey,
			Value: cookiev,
		},
	}
	oid, err := c.user(hcs)
	if err != nil {
		return nil, err
	}
	c.hcs = hcs
	c.orgid = oid
	if idv != "" {
		c.chatid = idv
	}

	return c.hcs, nil
}

func (c *web) run() {
	_, err := c.cookie()
	if err != nil {
		klog.Infof("claude cookie failed: %v", err)
	}

	for {
		select {
		case <-time.NewTimer(interval).C:
			c.cookie()
		case <-c.ctx.Done():
			return
		}
	}
}
