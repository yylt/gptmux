package deepseek

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/go-resty/resty/v2"
	"github.com/tmc/langchaingo/llms"
	"github.com/yylt/gptmux/mux"
	"github.com/yylt/gptmux/pkg"
	"github.com/yylt/gptmux/pkg/util"
	"k8s.io/klog/v2"
)

var (
	headers = map[string]string{
		"Host":         "chat.deepseek.com",
		"Origin":       "https://chat.deepseek.com",
		"accept":       "*/*",
		"content-type": "application/json",
		"User-Agent":   "Mozilla/5.0 (Linux; x64) Gecko/20100101 Firefox/128.0",
	}
	defaultClient = http.Client{
		Transport: http.DefaultTransport,
	}
)

type tokenResp struct {
	Data struct {
		User struct {
			Id    string `json:"id,omitempty"`
			Token string `son:"toen,omitempty"`
		} `json:"user,omitempty"`
	} `json:"data,omitempty"`
}

type Conf struct {
	// chat
	Email    string `yaml:"email"`
	Password string `yaml:"password"`
	DeviceId string `yaml:"deviceid"`
	Debug    bool   `yaml:"debug,omitempty"`
	Index    int    `yaml:"index,omitempty"`
}

type uuidResp struct {
	Data struct {
		BizData struct {
			Id    string `json:"id,omitempty"`
			Agent string `json:"agent,omitempty"`
		} `json:"biz_data,omitempty"`
	} `json:"data,omitempty"`
}

type Dseek struct {
	c  *Conf
	mu sync.Mutex

	rest *resty.Client

	token string
}

func New(c *Conf) *Dseek {
	if c == nil || c.Email == "" || c.Password == "" {
		klog.Warningf("deepseek config is invalid: %v", c)
		return nil
	}
	seek := &Dseek{
		c:    c,
		rest: resty.New(),
	}
	err := seek.login()
	if err != nil {
		klog.Errorf("%s: login failed: %v", seek.Name(), err)
		return nil
	}
	return seek
}

func (d *Dseek) Name() string {
	return "deepseek"
}

func (d *Dseek) Index() int {
	return d.c.Index
}

func (d *Dseek) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	if !d.mu.TryLock() {
		return nil, pkg.BusyErr
	}
	defer d.mu.Unlock()
	prompt, model := mux.GeneraPrompt(messages)

	if model != mux.TxtModel {
		return nil, fmt.Errorf("not support model '%s'", model)
	}
	uuid, err := d.newChat(d.token)
	if err != nil {
		if err := d.login(); err != nil {
			klog.Errorf("login failed: %s", err)
			return nil, err
		} else {
			uuid, _ := d.newChat(d.token)
			if uuid == "" {
				return nil, fmt.Errorf("can not chat")
			}
		}
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
	resp, err := d.chat(prompt, uuid)
	if err != nil {
		return nil, err
	}

	var (
		respData = &pkg.ChatResp{}

		ret  = &pkg.BackResp{}
		body = resp.Body
		once sync.Once
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
			klog.Warningf("parse event data failed: %v", err)
			continue
		}
		if d.c.Debug {
			klog.Infof("data: %s", string(bytes.TrimPrefix(line, util.HeaderData)))
		}
		ret.Content = ""
		ret.Err = nil

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
			once.Do(cancle)
		}
		if opt.StreamingFunc != nil {
			err = opt.StreamingFunc(bctx, []byte(ret.Content))
			if err != nil {
				break
			}
		}
	}

	return data, nil
}

func (d *Dseek) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	return "", fmt.Errorf("not implement")
}

func (d *Dseek) login() error {
	var (
		url                 = "https://chat.deepseek.com/api/v0/users/login"
		data                = &tokenResp{}
		req  *resty.Request = d.rest.R()
	)

	if d.c.Debug {
		req = req.SetDebug(true)
	}
	resp, err := req.SetBody(map[string]any{
		"email": d.c.Email, "password": d.c.Password,
		"mobile": "", "area_code": "",
		"device_id": d.c.DeviceId, "os": "web",
	}).SetHeaders(headers).SetResult(data).Post(url)
	if err != nil {
		return err
	}
	if !util.IsHttp20xCode(resp.StatusCode()) {
		return errors.Join(http.ErrNotSupported, fmt.Errorf("%s freshToken failed, http code %v", d.Name(), resp.StatusCode()))
	}
	d.token = data.Data.User.Token
	return nil
}

func (d *Dseek) chat(prompt string, uuid string) (*http.Response, error) {
	var url = "https://chat.deepseek.com/api/v0/chat/completion"
	// send prompt
	body := map[string]any{
		"prompt":            prompt,
		"parent_message_id": nil,
		"chat_session_id":   uuid,
		"ref_file_ids":      []string{},
		"thinking_enabled":  false,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("json failed"), err)
	}
	if d.c.Debug {
		klog.Infof("request body: %s", string(data))
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+d.token)

	// Add generic headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := defaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, fmt.Errorf("chat deepseek failed: %s, code: %v", http.StatusText(resp.StatusCode), resp.StatusCode)
	}
	return resp, err
}

func (d *Dseek) newChat(token string) (string, error) {
	var url = "https://chat.deepseek.com/api/v0/chat_session/create"
	if token == "" {
		return "", fmt.Errorf("token is null")
	}
	var (
		req  *resty.Request = d.rest.R()
		data                = &uuidResp{}
	)

	if d.c.Debug {
		req = req.SetDebug(true)
	}
	resp, err := req.SetBody(map[string]any{
		"agent": "chat",
	}).SetHeaders(headers).SetHeader("Authorization", fmt.Sprintf("Bearer %s", token)).SetResult(data).Post(url)
	if err != nil {
		return "", errors.Join(io.EOF, err)
	}
	if !util.IsHttp20xCode(resp.StatusCode()) {
		if resp.StatusCode()/100 == 4 {
			return "", errors.Join(http.ErrNotSupported, fmt.Errorf("http code %v", resp.StatusCode()))
		}
	}
	if data.Data.BizData.Id == "" {
		return "", fmt.Errorf("not found uuid")
	}
	return data.Data.BizData.Id, nil
}
