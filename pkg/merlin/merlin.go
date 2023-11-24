package merlin

import (
	"fmt"
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
)

type User struct {
	User     string `yaml:"name"`
	Password string `yaml:"password"`
}

type Config struct {
	Authurl string  `yaml:"authurl"`
	Appurl  string  `yaml:"appurl"`
	Users   []*User `yaml:"users"`
}

type cacheUser struct {
	accesstoken string
	name        string
	password    string
	used        int
	limit       int
}

func (c *cacheUser) DeepCopy() *cacheUser {
	return &cacheUser{
		accesstoken: c.accesstoken,
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
	ml := &Merlin{
		cfg:     cfg,
		cli:     cli,
		authurl: authurl,
		appurl:  appurl,
		read:    NewRespRead(),
	}
	go ml.run()
	return ml
}

// check users, and refresh token when forbidden
func (m *Merlin) run() {
	var interval = time.Minute * 37 // TODO
	for {
		for _, user := range m.cfg.Users {
			m.refresh(user)
		}
		<-time.Tick(interval)
	}
}

// check token or update
func (m *Merlin) refresh(u *User) {

	m.mu.RLock()
	v, ok := m.users[u.User]
	m.mu.RUnlock()
	if ok && v.accesstoken != "" {
		uc := v.DeepCopy()
		err := m.getStatus(uc)
		if err == nil {
			// update cache
			m.mu.Lock()
			defer m.mu.Unlock()
			m.users[uc.name] = uc
			return
		}
		// log
		klog.Errorf("merlin get status failed:%v", err)
		return
	}
	cache, err := m.access(u)
	if err != nil {
		klog.Error(err)
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[u.User] = cache
}

// get status and update cache.
func (m *Merlin) getStatus(cache *cacheUser) error {

	resp, err := m.cli.R().
		SetHeaders(HeaderDefault).
		SetHeader("accept", "*/*").
		SetBearerAuthToken(cache.accesstoken).
		Get(getStatusUrl(m.cfg.Appurl, cache.accesstoken))
	if err != nil {
		return err
	}
	if !pkg.IsHttp20xCode(resp.StatusCode) {
		resp.Body.Close()
		if resp.StatusCode == http.StatusUnauthorized {
			return UnauthErr
		}
		return fmt.Errorf("could not get merlin status: %s", http.StatusText(resp.StatusCode))
	}
	resp.Body.Close()

	var (
		status = merelinResp{}
	)
	err = resp.UnmarshalJson(&status)
	if err != nil {
		return err
	}
	ud, ok := status.Data.(*UserData)
	if !ok {
		return fmt.Errorf("response is not user data struct")
	}
	cache.used = ud.User.Used
	cache.limit = ud.User.Limit

	return nil
}

func (m *Merlin) access(u *User) (*cacheUser, error) {
	idtoken, err := m.idtoken(u)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	defer resp.Body.Close()

	if !pkg.IsHttp20xCode(resp.StatusCode) {
		resp.Body.Close()
		return nil, fmt.Errorf("could not get merlin access token: %s", http.StatusText(resp.StatusCode))
	}

	var (
		merlinrsp = merelinResp{}
	)
	err = resp.UnmarshalJson(&merlinrsp)
	if err != nil {
		return nil, err
	}
	ud, ok := merlinrsp.Data.(*tokenData)
	if !ok {
		return nil, fmt.Errorf("response is not token data struct")
	}

	cache := &cacheUser{
		accesstoken: ud.Access,
		name:        u.User,
		password:    u.Password,
	}
	err = m.getStatus(cache)
	if err != nil {
		return nil, err
	}
	return cache, nil
}

func (m *Merlin) idtoken(u *User) (string, error) {
	// idtoken
	resp, err := m.cli.R().
		SetHeaders(HeaderDefault).
		SetHeader("accept", "*/*").
		SetHeader("content-type", "application/json").
		SetHeader("authority", m.authurl.Host).
		SetBody(getAuth1Body(u.User, u.Password)).
		Post(getAuth1Url(m.cfg.Authurl))
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
	switch t {
	case pkg.TextGpt3:
		model = "GPT 3"
	case pkg.Code:
		model = "codellama/CodeLlama-34b-Instruct-hf"
	default:
		return nil, fmt.Errorf("not support prompt type")
	}

	return m.send(prompt, model)
}

// check and refresh token
// TODO cost count check
func (m *Merlin) getUser() *cacheUser {
	var (
		err      error
		newcache *cacheUser
	)

	m.mu.RLock()

	// depend on map random
	for _, v := range m.users {
		err = m.getStatus(v)
		if err != nil {
			if err == UnauthErr {
				newcache, err = m.access(&User{
					User:     v.name,
					Password: v.password,
				})
			}
			if err != nil {
				klog.Errorf("access token or get status failed: %v", err)
			}
			m.mu.RUnlock()
			return nil
		}
		m.mu.RUnlock()
		if newcache != nil {
			m.mu.Lock()
			defer m.mu.Unlock()
			m.users[newcache.name] = newcache
			v = newcache
		}
		return v.DeepCopy()
	}
	return nil
}

// model
func (m *Merlin) Model() []pkg.PromptType {
	return []pkg.PromptType{
		pkg.TextGpt3,
		pkg.Code,
	}
}

func (m *Merlin) send(prompt, model string) (<-chan *pkg.BackResp, error) {
	cu := m.getUser()
	if cu == nil {
		return nil, fmt.Errorf("there are no valid token to use")
	}
	// send prompt
	resp, err := m.cli.R().
		SetHeaders(HeaderDefault).
		SetHeader("accept", "text/event-stream").
		SetBearerAuthToken(cu.accesstoken).
		SetHeader("content-type", "application/json").
		SetHeader("authority", m.authurl.Host).
		SetBody(getContentBody(prompt, model)).
		Post(getContentUrl(m.cfg.Appurl))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		//		klog.Errorf("could not connect to stream: %s", http.StatusText(resp.StatusCode))
		return nil, fmt.Errorf("could not connect to stream: %s", http.StatusText(resp.StatusCode))
	}

	return m.read.Reader(resp.Body), nil
}
