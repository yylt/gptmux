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
	"time"

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

	instctrl *instCtrl
}

func NewMerlinIns(cfg *Config) *Merlin {
	if cfg == nil || cfg.Users == nil || cfg.Authurl == "" || cfg.Appurl == "" {
		klog.Errorf("merlin config is invalid: %v", cfg)
		return nil
	}
	ml := &Merlin{
		cfg: cfg,
	}
	defaultClient = util.NewDebugHTTPClient(cfg.Proxy, cfg.Debug)

	ml.instctrl = NewInstControl(time.Minute*55, ml, cfg.Users)
	if ml.instctrl.Size() == 0 {
		return nil
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
	err = m.chat(prompt, mux.TxtModel, func(resp *http.Response) error {
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

	err := m.chat(prompt, model, func(resp *http.Response) error {
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

// check token or update
func (m *Merlin) refresh(v *instance) error {
	err := m.access(v)
	if err == nil {
		err = m.usage(v)
	}
	if err == nil {
		klog.Infof("access success: %s", v)
	}
	return err
}

// update ins usage
func (m *Merlin) usage(ins *instance) error {
	var (
		status = &UserResp{}
		surl   = fmt.Sprintf("%s/status?firebaseToken=%s&from=DASHBOARD", m.cfg.Appurl, ins.idtoken)
	)

	resp, err := request(surl, "GET", nil, map[string]string{
		"accept":        "*/*",
		"Authorization": "Bearer " + ins.idtoken,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(status)
	if err != nil {
		return err
	}

	ins.used = status.Data.User.Used
	ins.limit = status.Data.User.Limit

	return nil
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

// check and refresh token
func (m *Merlin) getInstance(least int) (inst *instance, err error) {
	var (
		failed []*instance
	)
	for {
		inst, err = m.instctrl.Dequeue()
		if err != nil {
			klog.Errorf("merlin instance failed: %v", err)
			continue
		}
		if m.usage(inst) != nil {
			err = m.refresh(inst)
			if err != nil {
				klog.Errorf("merlin instance '%s' failed: %v", inst.user, err)
				failed = append(failed, inst)
				continue
			}
		}
		break
	}
	for _, v := range failed {
		m.instctrl.Eequeue(v)
	}
	return
}

func (m *Merlin) chat(prompt string, mode mux.ChatModel, fn func(*http.Response) error) error {

	var (
		url   string
		body  map[string]any
		least int
	)
	switch mode {
	case mux.TxtModel:
		url = fmt.Sprintf("%s/thread/unified?version=1.1", m.cfg.Appurl)
		body = chatBody(prompt, m.cfg.textModel())
		least = 1
	case mux.ImgModel:
		url = fmt.Sprintf("%s/thread/image-generation", m.cfg.Appurl)
		body = imageBody(prompt, m.cfg.imageModel())
		least = 10
	default:
		return fmt.Errorf("not support prompt type '%s'", mode)
	}
	cu, err := m.getInstance(least)
	if err != nil {
		return err
	}
	defer func() {
		m.usage(cu)
		klog.Infof("merlin user(%s) used %d, limit %d", cu.user, cu.used, cu.limit)
		m.instctrl.Eequeue(cu)
	}()

	bodystr, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal body failed :%v", err)
	}
	resp, err := request(url, "post", bodystr, map[string]string{
		"Accept":        "text/event-stream",
		"Connection":    "keep-alive",
		"content-type":  "application/json",
		"Authorization": "Bearer " + cu.idtoken,
	})
	if err != nil {
		return err
	}
	return fn(resp)
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
		if er.Data.Setting.Id != "" || er.Data.Usage.Type != "" {
			return nil
		}
		if len(er.Data.Attachs) != 0 && er.Data.Attachs[0].Url != "" {
			return &pkg.BackResp{
				Content: fmt.Sprintf("![](%s)", er.Data.Attachs[0].Url),
			}
		}
	}
	return nil
}
