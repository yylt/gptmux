package merlin

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"sync"

	"github.com/emirpasic/gods/queues/priorityqueue"
	"github.com/tmc/langchaingo/llms"
	"github.com/yylt/gptmux/pkg/mux"
	"github.com/yylt/gptmux/pkg/response"
	"github.com/yylt/gptmux/pkg/util"
	"k8s.io/klog/v2"
)

var _ mux.Model = &Merlin{}

type modelfn func(er *EventResp) *response.BackendData

var (
	HeaderDefault = map[string]string{
		"user-agent":      "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36",
		"accept-language": "zh-CN,zh;q=0.9,en;q=0.8,zh-Hans;q=0.7",
	}

	defaultClient *http.Client

	ErrAuth = errors.New("unauth")

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

	mu     sync.Mutex
	queue  *priorityqueue.Queue
	users  map[string]*instance
	active map[string]int
}

type UserInfo struct {
	Name  string `json:"name" yaml:"name"`
	Used  int    `json:"used,omitempty" yaml:"-"`
	Limit int    `json:"limit,omitempty" yaml:"-"`
}

func NewMerlinIns(cfg *Config) *Merlin {
	if cfg == nil || cfg.Users == nil || cfg.Authurl == "" || cfg.Appurl == "" {
		klog.Errorf("merlin config is invalid: %v", cfg)
		return nil
	}
	ml := &Merlin{
		cfg:    cfg,
		queue:  priorityqueue.NewWith(instCompare),
		users:  make(map[string]*instance, len(cfg.Users)),
		active: make(map[string]int, len(cfg.Users)),
	}
	var err error
	defaultClient, err = util.NewDebugHTTPClient(cfg.Proxy, cfg.Debug)
	if err != nil {
		klog.Errorf("create http client failed: %v", err)
		return nil
	}
	for _, user := range cfg.Users {
		u := NewInstance(ml, user)
		if u != nil {
			klog.Infof("merlin instance %s created", u)
			ml.users[u.user] = u
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

func (m *Merlin) ListUsers() []UserInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	ret := make([]UserInfo, 0, len(m.users))
	for _, cfgUser := range m.cfg.Users {
		if cfgUser == nil || cfgUser.User == "" {
			continue
		}
		info := UserInfo{Name: cfgUser.User}
		if ins, ok := m.users[cfgUser.User]; ok {
			info.Used = ins.used
			info.Limit = ins.limit
		}
		ret = append(ret, info)
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Name < ret[j].Name
	})
	return ret
}

func (m *Merlin) AddUser(username, password string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if username == "" || password == "" {
		return fmt.Errorf("name and password are required")
	}
	if _, ok := m.users[username]; ok {
		return fmt.Errorf("user %q already exists", username)
	}
	ins := NewInstance(m, &user{User: username, Password: password})
	if ins == nil {
		return fmt.Errorf("create merlin instance failed for %q", username)
	}
	m.cfg.Users = append(m.cfg.Users, &user{User: username, Password: password})
	m.users[username] = ins
	m.rebuildQueueLocked()
	return nil
}

func (m *Merlin) UpdateUser(username, password string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if username == "" || password == "" {
		return fmt.Errorf("name and password are required")
	}
	if m.active[username] > 0 {
		return fmt.Errorf("user %q is busy", username)
	}
	idx := -1
	for i, u := range m.cfg.Users {
		if u != nil && u.User == username {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("user %q not found", username)
	}
	ins := NewInstance(m, &user{User: username, Password: password})
	if ins == nil {
		return fmt.Errorf("update merlin instance failed for %q", username)
	}
	m.cfg.Users[idx].Password = password
	m.users[username] = ins
	m.rebuildQueueLocked()
	return nil
}

func (m *Merlin) DeleteUser(username string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if username == "" {
		return fmt.Errorf("name is required")
	}
	if m.active[username] > 0 {
		return fmt.Errorf("user %q is busy", username)
	}
	if _, ok := m.users[username]; !ok {
		return fmt.Errorf("user %q not found", username)
	}
	delete(m.users, username)
	delete(m.active, username)
	users := make([]*user, 0, len(m.cfg.Users))
	for _, u := range m.cfg.Users {
		if u == nil || u.User == username {
			continue
		}
		users = append(users, u)
	}
	m.cfg.Users = users
	m.rebuildQueueLocked()
	return nil
}

func (m *Merlin) rebuildQueueLocked() {
	queue := priorityqueue.NewWith(instCompare)
	for _, ins := range m.users {
		if m.active[ins.user] > 0 {
			continue
		}
		queue.Enqueue(ins)
	}
	m.queue = queue
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

			ret  *response.BackendData
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

			ret  *response.BackendData
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
		turl = "https://uam.getmerlin.in/session/get"
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
		url = fmt.Sprintf("%s/v1/thread/unified", m.cfg.Appurl)
		body = unifiedChatBody(prompt, m.cfg.textModel())
	case mux.ImgModel:
		url = fmt.Sprintf("%s/thread/image-generation", m.cfg.Appurl)
		body = imageBody(prompt, m.cfg.imageModel())
	default:
		return fmt.Errorf("not support prompt type '%s'", mode)
	}
	m.mu.Lock()
	cu, ok := m.queue.Dequeue()
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("%s is busy", m.Name())
	}
	ins := cu.(*instance)
	m.active[ins.user]++
	m.mu.Unlock()

	defer func() {
		klog.Infof("merlin chat done, %s", ins)
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.active[ins.user] > 0 {
			m.active[ins.user]--
		}
		if _, ok := m.users[ins.user]; ok {
			m.queue.Enqueue(ins)
		}
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
	if err != nil && errors.Is(err, ErrAuth) {
		err = m.access(ins)
		if err == nil {
			sendheader["Authorization"] = "Bearer " + ins.idtoken
			resp, err = request(url, "post", bodystr, sendheader)
		}
	}
	if err != nil {
		return err
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
		klog.Errorf("request '%s' failed: %v, code: %d", address, http.StatusText(resp.StatusCode), resp.StatusCode)
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, ErrAuth
		}
		return nil, fmt.Errorf("request '%s' code: %s", address, http.StatusText(resp.StatusCode))
	}
	return resp, nil
}

func textProcess(er *EventResp) *response.BackendData {
	if er == nil || er.Data == nil {
		return nil
	}
	switch er.Data.Type {
	case string(chunk):
		return &response.BackendData{
			Content: er.Data.Content,
		}
	case string(done):
		return &response.BackendData{
			Err: errors.New("done"),
		}
	}
	return nil
}

func imageProcess(er *EventResp) *response.BackendData {
	if er == nil || er.Data == nil {
		return nil
	}
	switch er.Data.Type {
	case string(system):
		if len(er.Data.Attachs) != 0 && er.Data.Attachs[0].Url != "" {
			return &response.BackendData{
				Content: fmt.Sprintf("![](%s)", er.Data.Attachs[0].Url),
			}
		}
	}
	return nil
}
