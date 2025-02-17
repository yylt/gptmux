package merlin

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/emirpasic/gods/queues/priorityqueue"
	"github.com/tmc/langchaingo/llms"
	"github.com/yylt/gptmux/mux"
	"github.com/yylt/gptmux/pkg"
	"github.com/yylt/gptmux/pkg/util"
	"k8s.io/klog/v2"
)

var _ mux.Model = &Merlin{}

type modelfn func(er *EventResp) *pkg.BackResp

var (
	HeaderDefault = map[string]string{
		"user-agent":      "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36",
		"accept-language": "zh-CN,zh;q=0.9,en;q=0.8,zh-Hans;q=0.7",
	}

	defaultClient *http.Client

	name = "merlin"
)

type Mode struct {
	Name  mux.ChatModel
	Model string
}

type merlinModel struct {
	// openai
	name mux.ChatModel

	// merlin
	kind string

	url    string
	datafn modelfn
}

type user struct {
	User     string `yaml:"name"`
	Password string `yaml:"password"`
}

type model struct {
	Text string `yaml:"text,omitempty"`
	Img  string `yaml:"image,omitempty"`
}

type Config struct {
	Index   int     `yaml:"index,omitempty"`
	Proxy   string  `yaml:"proxy,omitempty"`
	Debug   bool    `yaml:"debug,omitempty"`
	Authurl string  `yaml:"authurl"`
	Appurl  string  `yaml:"appurl"`
	Users   []*user `yaml:"users"`
	Model   model   `yaml:"model,omitempty"`
}

func (c *Config) textModel() string {
	if c.Model.Text == "" {
		return defaultChatModel
	}
	return c.Model.Text
}

func (c *Config) imageModel() string {
	if c.Model.Img == "" {
		return defaultImageModel
	}
	return c.Model.Img
}

type Merlin struct {
	cfg *Config

	queue *priorityqueue.Queue
}

func NewMerlinIns(cfg *Config) *Merlin {
	if cfg == nil || cfg.Users == nil || cfg.Authurl == "" || cfg.Appurl == "" {
		klog.Errorf("merlin config is invalid: %v", cfg)
		return nil
	}
	ml := &Merlin{
		cfg:   cfg,
		queue: priorityqueue.NewWith(instCompare),
	}
	defaultClient = util.NewDebugHTTPClient(cfg.Proxy, cfg.Debug)

	for _, user := range cfg.Users {
		u := NewInstance(ml, user)
		if u != nil {
			klog.Infof("merlin instance %s created", u)
			ml.queue.Enqueue(u)
		}
	}

	return ml
}

func (m *Merlin) Name() string {
	return name
}

func (m *Merlin) Index() int {
	return m.cfg.Index
}

func (m *Merlin) Completion(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	var (
		opt          = &llms.CallOptions{}
		bctx, cancle = context.WithCancel(ctx)
		err          error
		buf          = util.GetBuf()
	)
	for _, o := range options {
		o(opt)
	}
	defer util.PutBuf(buf)
	err = m.chat(prompt, mux.TxtModel, func(resp *http.Response, ins *instance) error {
		var (
			respData = &EventResp{}

			ret  *pkg.BackResp
			err  error
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
			if respData.Data.Usage.Limit != 0 {
				ins.used = respData.Data.Usage.Used
				ins.limit = respData.Data.Usage.Limit
			}
			ret = textProcess(respData)
			if ret == nil {
				continue
			}
			buf.WriteString(ret.Content)
			if ret.Err != nil {
				once.Do(cancle)
			}
			if opt.StreamingFunc != nil {
				err = opt.StreamingFunc(bctx, []byte(ret.Content))
				if err != nil {
					once.Do(cancle)
					return err
				}
			}
		}
		return nil
	})
	return buf.String(), err
}

func (m *Merlin) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	var (
		opt          = &llms.CallOptions{}
		bctx, cancle = context.WithCancel(ctx)
		data         = &llms.ContentResponse{}
	)
	for _, o := range options {
		o(opt)
	}
	prompt, model := mux.GeneraPrompt(messages)

	err := m.chat(prompt, model, func(resp *http.Response, ins *instance) error {
		var (
			respData = &EventResp{}

			ret  *pkg.BackResp
			err  error
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
			if respData.Data.Usage.Limit != 0 {
				ins.used = respData.Data.Usage.Used
				ins.limit = respData.Data.Usage.Limit
			}
			if model == mux.TxtModel {
				ret = textProcess(respData)
			} else {
				ret = imageProcess(respData)
			}
			if ret == nil {
				continue
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
					once.Do(cancle)
					return err
				}
			}
		}

		return nil
	})
	return data, err
}

func (m *Merlin) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	return "", fmt.Errorf("not implement")
}

func (m *Merlin) access(ins *instance) error {
	var (
		status = &authResp{}
		body   = map[string]interface{}{
			"returnSecureToken": true,
			"email":             ins.user,
			"password":          ins.password,
			"clientType":        "CLIENT_TYPE_WEB",
		}
		surl = m.cfg.Authurl
	)
	// idtoken
	bodys, _ := json.Marshal(body)
	resp, err := request(surl, "POST", bodys, map[string]string{
		"accept":       "*/*",
		"content-type": "application/json",
	})
	if err != nil {
		klog.Errorf("access id failed: %v", err)
		return err
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(status)
	if err != nil {
		return err
	}

	var (
		tstatus = &TokenResp{}
		tbody   = map[string]interface{}{
			"token": status.IdToken,
		}
		turl = fmt.Sprintf("%s/session/get", m.cfg.Appurl)
	)

	// accesstoken
	bodys, _ = json.Marshal(tbody)
	appresp, err := request(turl, "POST", bodys, map[string]string{
		"accept":       "*/*",
		"content-type": "application/json",
	})
	if err != nil {
		klog.Errorf("access token failed: %v", err)
		return err
	}
	defer appresp.Body.Close()
	err = json.NewDecoder(appresp.Body).Decode(tstatus)
	if err != nil {
		return err
	}

	ins.accesstoken = tstatus.Data.Access
	ins.idtoken = status.IdToken

	return nil
}

func (m *Merlin) chat(prompt string, mode mux.ChatModel, fn func(*http.Response, *instance) error) error {

	var (
		url  string
		body map[string]any
	)
	switch mode {
	case mux.TxtModel:
		url = fmt.Sprintf("%s/thread/unified?version=1.1", m.cfg.Appurl)
		body = chatBody(prompt, m.cfg.textModel())
	case mux.ImgModel:
		url = fmt.Sprintf("%s/thread/image-generation", m.cfg.Appurl)
		body = imageBody(prompt, m.cfg.imageModel())
	default:
		return fmt.Errorf("not support prompt type '%s'", mode)
	}
	cu, ok := m.queue.Dequeue()
	if !ok {
		return fmt.Errorf("%s is busy", m.Name())
	}
	ins := cu.(*instance)

	defer func() {
		klog.Infof("merlin chat done, %s", ins)
		m.queue.Enqueue(ins)
	}()

	bodystr, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal body failed :%v", err)
	}
	sendheader := map[string]string{
		"Accept":        "text/event-stream",
		"Connection":    "keep-alive",
		"content-type":  "application/json",
		"Authorization": "Bearer " + ins.idtoken,
	}
	resp, err := request(url, "post", bodystr, sendheader)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		err = m.access(ins)
		if err != nil {
			return err
		} else {
			resp, err = request(url, "post", bodystr, sendheader)
			if err != nil {
				return err
			}
		}
	}
	return fn(resp, ins)
}

func request(address, method string, body []byte, headers map[string]string) (*http.Response, error) {
	// send prompt
	var buf = &bytes.Buffer{}
	if body != nil {
		buf = bytes.NewBuffer(body)
	}
	req, err := http.NewRequest(method, address, buf)
	if err != nil {
		return nil, err
	}

	// Add Default headers
	for k, v := range HeaderDefault {
		req.Header.Set(k, v)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := defaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if !util.IsHttp20xCode(resp.StatusCode) {
		resp.Body.Close()
		return nil, fmt.Errorf("request '%s' failed: %v, code: %d", address, http.StatusText(resp.StatusCode), resp.StatusCode)
	}
	return resp, nil
}

func textProcess(er *EventResp) *pkg.BackResp {
	if er == nil || er.Data == nil {
		return nil
	}
	switch er.Data.Type {
	case string(chunk):
		return &pkg.BackResp{
			Content: er.Data.Content,
		}
	case string(done):
		return &pkg.BackResp{
			Err: errors.New("done"),
		}
	}
	return nil
}

func imageProcess(er *EventResp) *pkg.BackResp {
	if er == nil || er.Data == nil {
		return nil
	}
	switch er.Data.Type {
	case string(system):
		if len(er.Data.Attachs) != 0 && er.Data.Attachs[0].Url != "" {
			return &pkg.BackResp{
				Content: fmt.Sprintf("![](%s)", er.Data.Attachs[0].Url),
			}
		}
	}
	return nil
}
