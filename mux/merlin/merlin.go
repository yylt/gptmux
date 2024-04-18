package merlin

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	req "github.com/imroc/req/v3"
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

	defaultClient = http.Client{
		Transport: http.DefaultTransport,
	}

	name = "merlin"
)

type Mode struct {
	Name  pkg.ChatModel
	Model string
}

type merlinModel struct {
	// openai
	name pkg.ChatModel

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
	Gpt3 string `yaml:"gpt3,omitempty"`
	Gpt4 string `yaml:"gpt4,omitempty"`
	Img  string `yaml:"image,omitempty"`
}

type Config struct {
	Authurl string  `yaml:"authurl"`
	Authkey string  `yaml:"authkey"`
	Appurl  string  `yaml:"appurl"`
	Users   []*user `yaml:"users"`
	Debug   bool    `yaml:"debug"`
	Model   model   `yaml:"model,omitempty"`
}

func (c *Config) gpt3Model() string {
	if c.Model.Gpt3 == "" {
		return defaultChatModel
	}
	return c.Model.Gpt3
}
func (c *Config) gpt4Model() string {
	if c.Model.Gpt4 == "" {
		return defaultChatModel
	}
	return c.Model.Gpt4
}
func (c *Config) imageModel() string {
	if c.Model.Img == "" {
		return defaultImageModel
	}
	return c.Model.Img
}

type Merlin struct {
	cfg *Config

	cli *req.Client

	// gpt3Model string
	// gpt4Model string
	// imgModel  string

	authurl *url.URL
	appurl  *url.URL

	instctrl *instCtrl
}

func NewMerlinIns(cfg *Config) *Merlin {
	if cfg == nil || cfg.Users == nil {
		klog.Errorf("merlin config is invalid, user is null")
		return nil
	}
	appurl, err := util.ParseUrl(cfg.Appurl)
	if err != nil {
		panic(err)
	}
	authurl, err := util.ParseUrl(cfg.Appurl)
	if err != nil {
		panic(err)
	}

	ml := &Merlin{
		cli:     req.NewClient(),
		cfg:     cfg,
		authurl: authurl,
		appurl:  appurl,
	}
	ml.instctrl = NewInstControl(time.Minute*55, ml, cfg.Users)
	if p := util.GetEnvAny("HTTP_PROXY", "http_proxy"); p != "" {
		ml.cli.SetProxyURL(p)
	}
	if cfg.Debug {
		ml.cli = ml.cli.EnableDebugLog()
	}
	return ml
}

func (m *Merlin) Name() string {
	return name
}

func (m *Merlin) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	prompt, model := mux.GeneraPrompt(messages)
	klog.V(2).Infof("upstream '%s', model: %s, prompt %s", m.Name(), model, strconv.Quote(prompt))
	var (
		opt          = &llms.CallOptions{}
		bctx, cancle = context.WithCancel(ctx)
		data         = &llms.ContentResponse{}
	)
	for _, o := range options {
		o(opt)
	}

	err := m.chat(prompt, model, func(resp *http.Response) error {
		if resp.StatusCode != 200 {
			resp.Body.Close()
			return fmt.Errorf("chat failed: %s, code: %v", http.StatusText(resp.StatusCode), resp.StatusCode)
		}
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
			if model == pkg.TxtModel {
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
	if err != nil {
		klog.Errorf("get merlin access token failed: %v", err)
		return err
	}
	return m.usage(v)
}

// update ins usage
func (m *Merlin) usage(ins *instance) error {
	surl := fmt.Sprintf("https://%s/status?firebaseToken=%s&from=DASHBOARD", m.cfg.Appurl, ins.idtoken)

	resp, err := m.cli.R().
		SetHeaders(HeaderDefault).
		SetHeader("accept", "*/*").
		SetHeader("authority", m.authurl.Host).
		SetBearerAuthToken(ins.idtoken).
		Get(surl)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if !util.IsHttp20xCode(resp.StatusCode) {
		klog.Infof("usage failed, http code: %d", resp.StatusCode)
		if resp.StatusCode == http.StatusUnauthorized {
			return errors.New("unauth")
		}
		return fmt.Errorf("status get http code: %v", resp.StatusCode)
	}

	var (
		status = UserResp{}
	)
	err = resp.UnmarshalJson(&status)
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
		surl = fmt.Sprintf("https://%s/v1/accounts:signInWithPassword?key=%s", m.cfg.Authurl, m.cfg.Authkey)
	)
	// idtoken
	resp, err := m.cli.R().
		SetHeaders(HeaderDefault).
		SetHeader("accept", "*/*").
		SetHeader("content-type", "application/json").
		SetHeader("authority", m.authurl.Host).
		SetResult(status).
		SetBody(body).
		Post(surl)
	if err != nil {
		return err
	}
	resp.Response.Body.Close()

	var (
		tstatus = &TokenResp{}
		tbody   = map[string]interface{}{
			"token": status.IdToken,
		}
		turl = fmt.Sprintf("https://%s/session/get", m.cfg.Appurl)
	)

	// accesstoken
	resp, err = m.cli.R().
		SetHeaders(HeaderDefault).
		SetHeader("accept", "*/*").
		SetHeader("content-type", "application/json").
		SetHeader("authority", m.authurl.Host).
		SetBody(tbody).
		SetResult(tstatus).
		Post(turl)
	if err != nil {
		return err
	}
	resp.Response.Body.Close()

	ins.accesstoken = tstatus.Data.Access
	ins.idtoken = status.IdToken
	return nil
}

func (m *Merlin) Send(prompt string, model pkg.ChatModel) (<-chan *pkg.BackResp, error) {
	var rsch = make(chan *pkg.BackResp, 16)

	go m.chat(prompt, model, func(resp *http.Response) error {
		if resp.StatusCode != 200 {
			resp.Body.Close()
			close(rsch)
			return fmt.Errorf("chat failed: %s, code: %v", http.StatusText(resp.StatusCode), resp.StatusCode)
		}
		var (
			respData = &EventResp{}

			ret  *pkg.BackResp
			err  error
			body = resp.Body
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
			if model == pkg.TxtModel {
				ret = textProcess(respData)
			} else {
				ret = imageProcess(respData)
			}
			if ret == nil {
				continue
			}
			rsch <- ret
		}
		rsch <- &pkg.BackResp{
			Err: scanner.Err(),
		}
		close(rsch)
		return nil
	})
	return rsch, nil
}

// check and refresh token
func (m *Merlin) getInstance(least int) (*instance, error) {
	var (
		err error
	)
	inst, err := m.instctrl.Dequeue()
	if err != nil {
		klog.Errorf("merlin get instance failed: %v", err)
		return nil, err
	}
	if m.usage(inst) != nil {
		err = m.refresh(inst)
		if err != nil {
			klog.Errorf("merlin refresh instance failed: %v", err)
			return nil, err
		}
	}
	return inst, err
}

func (m *Merlin) chat(prompt string, mode pkg.ChatModel, fn func(*http.Response) error) error {

	var (
		url   string
		body  map[string]any
		least int
	)
	switch mode {
	case pkg.TxtModel:
		url = fmt.Sprintf("https://%s/thread?customJWT=true&version=1.1", m.cfg.Appurl)
		body = chatBody(prompt, m.cfg.gpt4Model())
		least = 1
	case pkg.ImgModel:
		url = fmt.Sprintf("https://%s/thread/image-generation?customJWT=true", m.cfg.Appurl)
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
	// send prompt
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(bodystr))
	if err != nil {
		return err
	}

	req.Header.Set("authority", m.authurl.Host)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("content-type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cu.accesstoken)

	// Add generic headers
	for k, v := range HeaderDefault {
		req.Header.Set(k, v)
	}

	resp, err := defaultClient.Do(req)

	if err != nil {
		return err
	}
	err = fn(resp)
	return err
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
