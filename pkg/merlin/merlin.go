package merlin

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	req "github.com/imroc/req/v3"
	"github.com/yylt/chatmux/pkg"
	"github.com/yylt/chatmux/pkg/util"
	"k8s.io/klog/v2"
)

var _ pkg.Backender = &Merlin{}

type modelfn func(er *EventResp) *pkg.BackResp

var (
	HeaderDefault = map[string]string{
		"user-agent":      "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36",
		"accept-language": "zh-CN,zh;q=0.9,en;q=0.8,zh-Hans;q=0.7",
	}

	defaultClient = http.Client{
		Transport: http.DefaultTransport,
	}
	ErrUnauth = errors.New("unauth")
)

type Mode struct {
	Name  pkg.ChatModel
	Model string
}

type User struct {
	User     string `yaml:"name"`
	Password string `yaml:"password"`
}
type Model struct {
	Gpt3 string `yaml:"gpt3,omitempty"`
	Gpt4 string `yaml:"gpt4,omitempty"`
	Img  string `yaml:"image,omitempty"`
}

type Config struct {
	Authurl string  `yaml:"authurl"`
	Authkey string  `yaml:"authkey"`
	Appurl  string  `yaml:"appurl"`
	Users   []*User `yaml:"users"`
	Debug   bool    `yaml:"debug"`
	Model   Model   `yaml:"model,omitempty"`
}

type Merlin struct {
	cfg *Config

	cli *req.Client

	authurl *url.URL
	appurl  *url.URL

	instctrl *instCtrl
}

func NewMerlinIns(cfg *Config) *Merlin {
	if cfg == nil || cfg.Users == nil {
		panic("merlin config is invalid, user is null")
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

	if cfg.Debug {
		ml.cli = ml.cli.EnableDebugLog()
	}
	return ml
}

// check token or update
func (m *Merlin) refresh(v *instance) error {
	err := m.access(v)
	if err != nil {
		klog.Errorf("get merlin access token failed: %v", err)
		return err
	}
	return m.status(v)
}

// get status and update cache.
// return error
func (m *Merlin) status(cache *instance) error {
	resp, err := m.cli.R().
		SetHeaders(HeaderDefault).
		SetHeader("accept", "*/*").
		SetHeader("authority", m.authurl.Host).
		SetBearerAuthToken(cache.idtoken).
		Get(getStatusUrl(m.cfg.Appurl, cache.idtoken))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if !util.IsHttp20xCode(resp.StatusCode) {

		if resp.StatusCode == http.StatusUnauthorized {
			return ErrUnauth
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
	cache.used = status.Data.User.Used
	cache.limit = status.Data.User.Limit

	return nil
}

func (m *Merlin) access(u *instance) error {
	idtoken, err := m.idtoken(u)
	if err != nil {
		return fmt.Errorf("get id token failed :%v", err)
	}
	// accesstoken
	resp, err := m.cli.R().
		SetHeaders(HeaderDefault).
		SetHeader("accept", "*/*").
		SetHeader("content-type", "application/json").
		SetHeader("authority", m.authurl.Host).
		SetBody(getAuth2Body(idtoken)).
		Post(getAuth2Url(m.cfg.Appurl))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if !util.IsHttp20xCode(resp.StatusCode) {
		resp.Body.Close()
		return fmt.Errorf("merlin could not get access token: %s", http.StatusText(resp.StatusCode))
	}

	var (
		merlinrsp = TokenResp{}
	)

	err = resp.UnmarshalJson(&merlinrsp)
	if err != nil {
		return err
	}
	u.accesstoken = merlinrsp.Data.Access
	u.idtoken = idtoken
	return nil
}

func (m *Merlin) idtoken(u *instance) (string, error) {
	// idtoken
	resp, err := m.cli.R().
		SetHeaders(HeaderDefault).
		SetHeader("accept", "*/*").
		SetHeader("content-type", "application/json").
		SetHeader("authority", m.authurl.Host).
		SetBody(getAuth1Body(u.user, u.password)).
		Post(getAuth1Url(m.cfg.Authurl, m.cfg.Authkey))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if !util.IsHttp20xCode(resp.StatusCode) {
		resp.Body.Close()
		return "", fmt.Errorf("could not get merlin istoken: %s", http.StatusText(resp.StatusCode))
	}

	var (
		status = authResp{}
	)
	err = resp.UnmarshalJson(&status)
	if err != nil {
		return "", err
	}
	return status.IdToken, nil
}

func (m *Merlin) Send(prompt string, t pkg.ChatModel) (<-chan *pkg.BackResp, error) {
	var (
		body any
		mode = Mode{
			Name:  t,
			Model: defaultChatModel,
		}
	)

	switch t {
	case pkg.GPT3Model, pkg.GPT3PlusModel:
		if m.cfg.Model.Gpt3 != "" {
			mode.Model = m.cfg.Model.Gpt3
		}
		body = chatBody(prompt, mode.Model)

	case pkg.GPT4Model, pkg.GPT4PlusModel:
		if m.cfg.Model.Gpt4 != "" {
			mode.Model = m.cfg.Model.Gpt4
		}
		body = chatBody(prompt, mode.Model)
	case pkg.ImgModel:
		if m.cfg.Model.Img != "" {
			mode.Model = m.cfg.Model.Img
		} else {
			mode.Model = defaultImageModel
		}
		body = imageBody(prompt, mode.Model)

	default:
		return nil, fmt.Errorf("not support prompt type \"%s\"", t)
	}

	return m.send(body, mode)
}

// check and refresh token
func (m *Merlin) getInstance(model string) (*instance, error) {
	var (
		err error
	)
	inst, err := m.instctrl.Dequeue(model)
	if err != nil {
		klog.Errorf("merlin get instance failed for model %s: %v", model, err)
		return nil, err
	}
	if m.status(inst) != nil {
		err = m.refresh(inst)
		if err != nil {
			klog.Errorf("merlin refresh instance failed: %v", err)
			return nil, err
		}
	}
	return inst, err
}

func (m *Merlin) send(body any, mod Mode) (<-chan *pkg.BackResp, error) {
	var (
		datafn modelfn
		url    string
	)

	switch mod.Name {
	case pkg.ImgModel:
		datafn = imageProcess
		url = getImageUrl(m.cfg.Appurl)
	default:
		datafn = textProcess
		url = getChatUrl(m.cfg.Appurl)
	}

	cu, err := m.getInstance(mod.Model)
	if err != nil {
		return nil, err
	}
	defer m.instctrl.Eequeue(cu)

	bodystr, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body failed :%v", err)
	}
	// send prompt
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(bodystr))
	if err != nil {
		return nil, err
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
		return nil, err
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, fmt.Errorf("could not connect to stream: %s", http.StatusText(resp.StatusCode))
	}
	var rsch = make(chan *pkg.BackResp, 4)
	go func(sch chan *pkg.BackResp, body io.ReadCloser) {
		var (
			respData = &EventResp{}
			err      error
		)
		defer body.Close()
		scanner := bufio.NewScanner(body)
		for scanner.Scan() {
			line := scanner.Bytes()

			if !bytes.HasPrefix(line, dataPrefix) {
				continue
			}
			err = json.Unmarshal(bytes.TrimPrefix(line, dataPrefix), &respData)
			if err != nil {
				klog.Warningf("parse event data failed: %v", err)
				continue
			}
			evresp := datafn(respData)
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

func textProcess(er *EventResp) *pkg.BackResp {
	if er == nil || er.Data == nil {
		return nil
	}
	switch er.Data.Type {
	case string(chunk):
		return &pkg.BackResp{
			Content: er.Data.Content,
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
		if er.Data.Setting != nil || er.Data.Usage != nil {
			return nil
		}
		if len(er.Data.Attachs) != 0 && er.Data.Attachs[0].Url != "" {
			return &pkg.BackResp{
				Content: er.Data.Attachs[0].Url,
			}
		}
	}
	return nil
}
