package coplit

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	bing "github.com/Harry-zklcdc/bing-lib"
	msauth "github.com/Harry-zklcdc/ms-auth"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/schema"
	"github.com/yylt/gptmux/pkg"
	"github.com/yylt/gptmux/pkg/box"
	"github.com/yylt/gptmux/pkg/util"
	"k8s.io/klog/v2"
)

const (
	bingBaseUrl   = "https://www.bing.com"
	sydneyBaseUrl = "wss://sydney.bing.com"
	bypassUrl     = "https://zklcdc-pass.hf.space"

	boxCookie = "coplit-cookie"
	boxCode   = "coplit-code"
)

type Config struct {
	Email string `json:"email"`
}

type copilot struct {
	chat *bing.Chat

	mu sync.RWMutex

	ctx context.Context

	bo box.Box

	authed bool

	email string
}

func New(c *Config, bo box.Box) *copilot {
	cp := &copilot{
		email: c.Email,
		bo:    bo,
		chat:  &bing.Chat{},
	}

	return cp
}

func (c *copilot) probe() {
	c.chat.SetBingBaseUrl(bingBaseUrl)
	c.chat.SetSydneyBaseUrl(sydneyBaseUrl)
	c.chat.SetStyle(bing.BALANCED)
	c.chat.SetBypassServer(bypassUrl)
}

// find cookie from box, newChat
func (c *copilot) cookie() (rerr error) {
	var (
		cookie string
	)
	rerr = c.bo.Receive(func(m *box.Message) bool {
		if m.Title == boxCookie {
			cookie = m.Msg
			return false
		}
		return true
	})
	defer func() {
		if rerr != nil {
			c.chat.SetCookies("")
		}
	}()
	if cookie != "" {
		rerr = c.chat.SetCookies(cookie).NewConversation()
		if rerr != nil {
			klog.Errorf("new conversation failed with coplit cookie: %v", cookie)
			return rerr
		}
	}
	return pkg.NotFoundErr
}

// no code will notify
// noerror mean set cookie
func (c *copilot) code() (err error) {
	var (
		send bool = true
		code string
	)

	err = c.bo.Receive(func(m *box.Message) bool {
		if m.Title == boxCode {
			code = strings.TrimSpace(m.Msg)
			if m.Time.Add(time.Hour * 24).After(time.Now()) {
				send = false
			}
			return false
		}
		return true
	})
	defer func() {
		if send {
			c.bo.Push(&box.Message{
				Title: fmt.Sprintf("%s require", boxCode),
				Msg:   "coplit require code",
			})
		}
	}()
	if err != nil {
		return err
	}
	if code != "" {
		auth := msauth.NewAuth(c.email, "", msauth.TYPE_EMAIL)
		cookies, err := auth.AuthEmail(code)
		if err != nil {
			return err
		}
		return c.bo.Push(&box.Message{
			Title: boxCookie,
			Msg:   cookies,
		})
	}
	return fmt.Errorf("not found code")
}

func (c *copilot) auth() error {
	klog.Infof("try authen to coplit")
	err := c.cookie()
	if err != nil {
		klog.Warningf("coplit cookit failed: %v", err)
		err = c.code()
		if err != nil {
			klog.Warningf("coplit code failed: %v", err)
			return err
		}
		return c.cookie()
	}
	return err
}

func (c *copilot) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	if c.mu.TryLock() == false {
		return nil, fmt.Errorf("wait")
	}
	defer c.mu.Unlock()
	if c.chat.GetCookies() == "" {
		err := c.auth()
		if err != nil {
			return nil, err
		}
	}
	var (
		opt = &llms.CallOptions{}
	)
	for _, o := range options {
		o(opt)
	}
	if opt.StreamingFunc != nil {

	}
	return nil, nil
}

func GetPrompt(messages []llms.MessageContent) string {
	buf := util.GetBuf()
	defer util.PutBuf(buf)
	if len(messages) < 1 {
		return ""
	}
	for _, msg := range messages {
		if msg.Role != schema.ChatMessageTypeHuman {
			continue
		}
		for _, p := range msg.Parts {
			txt, ok := p.(*llms.TextContent)
			if ok {
				buf.WriteString(txt.Text)
			}
		}
	}
	return buf.String()
}
