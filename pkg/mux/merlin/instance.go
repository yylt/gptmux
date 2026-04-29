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

	"github.com/yylt/gptmux/pkg/util"
	"k8s.io/klog/v2"
)

var (
	defaultImageModel = "Dreamshape v7"
	defaultChatModel  = "gpt-4o-mini"
	merlinAppUrl      = "https://uam.getmerlin.in"
	merlinAccessUrl   = "https://uam.getmerlin.in/session/get"
)

type instance struct {
	accesstoken string // chat
	idtoken     string
	user        string
	password    string
	used        int
	limit       int
}

func (c *instance) DeepCopy() *instance {
	return &instance{
		accesstoken: c.accesstoken,
		user:        c.user,
		password:    c.password,
		used:        c.used,
		limit:       c.limit,
	}
}
func (c *instance) String() string {
	return fmt.Sprintf("user: %s(%d/%d)", c.user, c.used, c.limit)
}

func instCompare(a, b interface{}) int {
	s1 := a.(*instance)
	s2 := b.(*instance)
	if s1.used > s2.used {
		return 1
	} else if s1.used == s2.used {
		return 0
	}
	return -1
}

func NewInstance(m *Merlin, u *user) *instance {
	ins := &instance{
		user:     u.User,
		password: u.Password,
	}
	err := m.access(ins)
	if err != nil {
		klog.Errorf("access user %s, error: %v", ins, err)
		return nil
	}
	return ins
}

type Instance struct {
	cfg *InstanceConfig

	used, limit          int
	idtoken, accesstoken string

	sync.RWMutex
}

type InstanceConfig struct {
	User, Password string
	Authurl        string
	Proxy          string
	Debug          bool
}

func NewInstance2(cfg *InstanceConfig) (*Instance, error) {
	if cfg == nil || cfg.User == "" || cfg.Password == "" || cfg.Authurl == "" {
		return nil, fmt.Errorf("invalid instance config: %v", cfg)
	}
	var err error
	defaultClient, err = util.NewDebugHTTPClient(cfg.Proxy, cfg.Debug)
	if err != nil {
		return nil, fmt.Errorf("create http client failed: %v", err)
	}
	return &Instance{
		cfg: cfg,
	}, nil
}

func (i *Instance) String() string {
	return fmt.Sprintf("user: %s(%d/%d)", i.cfg.User, i.used, i.limit)
}

func (ins *Instance) auth(ctx context.Context) error {
	var ()
	var (
		status = &authResp{}
		body   = map[string]interface{}{
			"returnSecureToken": true,
			"email":             ins.cfg.User,
			"password":          ins.cfg.User,
			"clientType":        "CLIENT_TYPE_WEB",
		}
		surl = ins.cfg.Authurl
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

func (inst *Instance) handler(ctx context.Context, ischat bool, resp *http.Response, fn func(done bool, content []byte)) error {
	var (
		respData = &EventResp{}
		err      error
		body     = resp.Body
	)
	if fn == nil {
		return fmt.Errorf("please provide a valid function to handle content")
	}
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			klog.Infof("context done, stop processing events")
			return ctx.Err()
		default:
			line := scanner.Bytes()
			if !bytes.HasPrefix(line, util.HeaderData) {
				continue
			}
			err = json.Unmarshal(bytes.TrimPrefix(line, util.HeaderData), &respData)
			if err != nil {
				klog.Warningf("parse event data failed: %v", err)
				continue
			}
			if respData.Data == nil {
				continue
			}
			if respData.Data.Usage.Limit != 0 {
				inst.used = respData.Data.Usage.Used
				inst.limit = respData.Data.Usage.Limit
			}
			if ischat {
				if respData.Data.Content != "" {
					fn(false, []byte(respData.Data.Content))
				}
				if respData.Data.Type == "DONE" {
					fn(true, nil)
					return nil
				}
			} else {
				if len(respData.Data.Attachs) > 0 {
					content := fmt.Sprintf("![](%s)", respData.Data.Attachs[0].Url)
					fn(true, []byte(content))
					return nil
				}
			}
		}
	}

	return scanner.Err()
}

func (inst *Instance) Chat(ctx context.Context, model, prompt string, fn func(done bool, content []byte)) error {
	var (
		url = fmt.Sprintf("%s/v1/thread/unified", merlinAppUrl)
	)
	var usemodel = model
	if usemodel == "" {
		usemodel = defaultChatModel
	}
	body, err := json.Marshal(unifiedChatBody(prompt, usemodel))
	if err != nil {
		return fmt.Errorf("marshal body failed :%v", err)
	}
	sendheader := map[string]string{
		"Accept":        "text/event-stream",
		"Connection":    "keep-alive",
		"content-type":  "application/json",
		"Authorization": "Bearer " + inst.idtoken,
	}
	resp, err := request(url, "post", body, sendheader)
	if err != nil && errors.Is(err, ErrAuth) {
		err = inst.auth(ctx)
		if err == nil {
			sendheader["Authorization"] = "Bearer " + inst.idtoken
			resp, err = request(url, "post", body, sendheader)
		}
	}
	if err != nil {
		return fmt.Errorf("request chat failed: %v", err)
	}
	defer resp.Body.Close()
	return inst.handler(ctx, true, resp, fn)
}

func (inst *Instance) Image(ctx context.Context, model, prompt string, fn func(done bool, content []byte)) error {
	var (
		url      = fmt.Sprintf("%s/thread/image-generation", merlinAppUrl)
		usemodel = model
	)
	if usemodel == "" {
		usemodel = defaultImageModel
	}
	body, err := json.Marshal(unifiedChatBody(prompt, usemodel))
	if err != nil {
		return fmt.Errorf("marshal body failed :%v", err)
	}
	sendheader := map[string]string{
		"Accept":        "text/event-stream",
		"Connection":    "keep-alive",
		"content-type":  "application/json",
		"Authorization": "Bearer " + inst.idtoken,
	}
	resp, err := request(url, "post", body, sendheader)
	if err != nil && errors.Is(err, ErrAuth) {
		err = inst.auth(ctx)
		if err == nil {
			sendheader["Authorization"] = "Bearer " + inst.idtoken
			resp, err = request(url, "post", body, sendheader)
		}
	}
	if err != nil {
		return fmt.Errorf("request chat failed: %v", err)
	}
	defer resp.Body.Close()
	return inst.handler(ctx, false, resp, fn)
}
