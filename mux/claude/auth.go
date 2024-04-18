package claude

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/yylt/gptmux/pkg/box"
	"k8s.io/klog/v2"
)

const (
	// check and notidy
	interval = time.Hour * 24

	cookiekey = "sessionKey"
)

type auth struct {
	mu sync.RWMutex
	b  box.Box

	c *web

	ctx context.Context

	lastupdate time.Time

	hcs []*http.Cookie
}

// 从box获取 token，并验证是否可用
// 不可用则通知
func (r *auth) cookie() ([]*http.Cookie, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.hcs != nil {
		_, err := r.c.user(r.hcs)
		if err == nil {
			return r.hcs, nil
		}
	}

	var (
		now  = time.Now()
		code string
	)

	err := r.b.Receive(func(m *box.Message) bool {
		if strings.ToLower(m.Title) != ClaudeName {
			return true
		}

		code = strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) {
				return rune(0)
			}
			return r
		}, m.Msg)

		klog.Infof("found msg %#v, and code: %s", m, code)
		return false
	})
	if err != nil {
		return nil, err
	}
	// not found
	if code == "" {
		if r.lastupdate.Add(interval).Before(now) {
			r.b.Push(&box.Message{
				Title: "require token: " + ClaudeName,
				Msg:   "更新token",
			})
		}
		return nil, fmt.Errorf("not found claude token from box")
	}
	r.hcs = []*http.Cookie{
		{
			Name:  cookiekey,
			Value: code,
		},
	}

	return r.hcs, nil
}

func (r *auth) run() {
	for {
		select {
		case <-time.NewTimer(interval).C:
			r.cookie()
		case <-r.ctx.Done():
			return
		}
	}
}

func newAuth(c *web, ctx context.Context, b box.Box) *auth {
	r := &auth{
		c:   c,
		ctx: ctx,
		b:   b,
	}
	return r
}
