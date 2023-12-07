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
	"sync"
	"time"

	req "github.com/imroc/req/v3"
	"github.com/yylt/chatmux/pkg"
	"k8s.io/klog/v2"
)

var _ pkg.Backender = &Merlin{}

var (
	HeaderDefault = map[string]string{
		"user-agent":      "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36",
		"accept-language": "zh-CN,zh;q=0.9,en;q=0.8,zh-Hans;q=0.7",
	}

	defaultClient = http.Client{
		Transport: http.DefaultTransport,
	}
	ErrUnauth = errors.New("unauth")

	supportPrompts = []pkg.PromptType{
		pkg.TextGpt3,
		pkg.TextGpt4,
	}
	defaultmodel = "claude-instant-1"
)

type User struct {
	User     string `yaml:"name"`
	Password string `yaml:"password"`
}
type Model struct {
	Gpt3 string `yaml:"gpt3"`
	Gpt4 string `yaml:"gpt4"`
}

type Config struct {
	Authurl string  `yaml:"authurl"`
	Authkey string  `yaml:"authkey"`
	Appurl  string  `yaml:"appurl"`
	Users   []*User `yaml:"users"`
	Debug   bool    `yaml:"debug"`
	Model   Model   `yaml:"model,omitempty"`
}

type cacheUser struct {
	idtoken     string // status
	accesstoken string // chat
	name        string
	password    string
	used        int
	limit       int
}

func (c *cacheUser) DeepCopy() *cacheUser {
	return &cacheUser{
		accesstoken: c.accesstoken,
		idtoken:     c.idtoken,
		name:        c.name,
		password:    c.password,
		used:        c.used,
		limit:       c.limit,
	}
}

type Merlin struct {
	cfg *Config

	cli *req.Client

	authurl *url.URL
	appurl  *url.URL

	mu sync.RWMutex
	// user : accesstoken
	users map[string]*cacheUser

	read *respReadClose
}

func NewMerlinIns(cfg *Config) *Merlin {
	if cfg == nil || cfg.Users == nil {
		panic("merlin config is invalid, user is null")
	}
	appurl, err := pkg.ParseUrl(cfg.Appurl)
	if err != nil {
		panic(err)
	}
	authurl, err := pkg.ParseUrl(cfg.Appurl)
	if err != nil {
		panic(err)
	}
	cli := req.NewClient()
	if cfg.Debug {
		cli.DebugLog = true
		cli = cli.EnableDumpAll()
	}

	ml := &Merlin{
		cfg:     cfg,
		cli:     cli,
		authurl: authurl,
		appurl:  appurl,
		read:    NewRespRead(),
		users:   map[string]*cacheUser{},
	}
	go ml.run()
	return ml
}

// check users, and refresh token when forbidden
func (m *Merlin) run() {
	var interval = time.Minute * 37 // TODO
	for {
		m.mu.Lock()
		for _, user := range m.cfg.Users {
			v, ok := m.users[user.User]
			if !ok {
				m.users[user.User] = &cacheUser{
					name:     user.User,
					password: user.Password,
				}
				v = m.users[user.User]
			}
			if v.accesstoken != "" && m.status(v) == nil {
				continue
			}
			err := m.refresh(v)
			if err != nil {
				klog.Errorf("user(%s/%s) could not get merlin access: %v", user.User, user.Password, err)
			}
		}
		m.mu.Unlock()

		<-time.NewTicker(interval).C
	}
}

// check token or update
func (m *Merlin) refresh(v *cacheUser) error {
	err := m.access(v)
	if err != nil {
		klog.Errorf("get merlin access token failed: %v", err)
		return err
	}
	return m.status(v)
}

// get status and update cache.
// return error
func (m *Merlin) status(cache *cacheUser) error {
	resp, err := m.cli.R().
		SetHeaders(HeaderDefault).
		SetHeader("accept", "*/*").
		SetHeader("authority", m.authurl.Host).
		SetBearerAuthToken(cache.idtoken).
		Get(getStatusUrl(m.cfg.Appurl, cache.idtoken))
	if err != nil {
		return err
	}
	if !pkg.IsHttp20xCode(resp.StatusCode) {
		resp.Body.Close()
		if resp.StatusCode == http.StatusUnauthorized {
			return ErrUnauth
		}
		return err
	}
	resp.Body.Close()

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

func (m *Merlin) access(u *cacheUser) error {
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

	if !pkg.IsHttp20xCode(resp.StatusCode) {
		resp.Body.Close()
		return fmt.Errorf("could not get merlin access token: %s", http.StatusText(resp.StatusCode))
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

func (m *Merlin) idtoken(u *cacheUser) (string, error) {
	// idtoken
	resp, err := m.cli.R().
		SetHeaders(HeaderDefault).
		SetHeader("accept", "*/*").
		SetHeader("content-type", "application/json").
		SetHeader("authority", m.authurl.Host).
		SetBody(getAuth1Body(u.name, u.password)).
		Post(getAuth1Url(m.cfg.Authurl, m.cfg.Authkey))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if !pkg.IsHttp20xCode(resp.StatusCode) {
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

func (m *Merlin) Send(prompt string, t pkg.PromptType) (<-chan *pkg.BackResp, error) {
	var model string
	model = defaultmodel
	switch t {
	case pkg.TextGpt3:
		if m.cfg.Model.Gpt3 != "" {
			model = m.cfg.Model.Gpt3
		}

	case pkg.TextGpt4:
		if m.cfg.Model.Gpt4 != "" {
			model = m.cfg.Model.Gpt4
		}
	default:
		return nil, fmt.Errorf("not support prompt type")
	}

	return m.send(prompt, model)
}

// check and refresh token
func (m *Merlin) getUser() *cacheUser {
	var (
		err error
	)

	m.mu.RLock()
	defer m.mu.RUnlock()
	// depend on map random
	for _, v := range m.users {
		err = m.status(v)
		klog.Errorf("get status failed: %v", err)
		if err != nil && err == ErrUnauth {
			err = m.refresh(v)
			if err != nil {
				klog.Errorf("refresh user(%s/%s) failed: %v", v.name, v.password, err)
				return nil
			}
		}
		return v.DeepCopy()
	}
	return nil
}

// model
func (m *Merlin) Model() []pkg.PromptType {
	return supportPrompts
}

func (m *Merlin) send(prompt, model string) (<-chan *pkg.BackResp, error) {
	cu := m.getUser()
	if cu == nil {
		return nil, fmt.Errorf("there are no valid user")
	}
	body, err := json.Marshal(getContentBody(prompt, model))
	if err != nil {
		return nil, fmt.Errorf("marshal body failed :%v", err)
	}
	// send prompt
	req, err := http.NewRequest("POST", getContentUrl(m.cfg.Appurl), bytes.NewBuffer(body))
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

			if !bytes.HasPrefix(line, dataMsg) {
				continue
			}
			err = json.Unmarshal(bytes.TrimPrefix(line, dataMsg), &respData)
			if err != nil {
				klog.Warningf("parse event data failed: %v", err)
				continue
			}

			evdata := respData.Data
			switch evdata.Type {
			case string(chunk):
				sch <- &pkg.BackResp{
					Content: evdata.Content,
				}
			}
		}
		sch <- &pkg.BackResp{
			Err: scanner.Err(),
		}
		close(sch)

	}(rsch, resp.Body)

	return rsch, nil
}
