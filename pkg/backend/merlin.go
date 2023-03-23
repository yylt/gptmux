package backend

import (
	"fmt"
	"io"

	req "github.com/imroc/req/v3"
	"github.com/yylt/chatmux/pkg"
)

// https://app.getmerlin.in

const (
	authurl  = ""
	queryurl = ""
)

var (
	defaultMaxConCurrent int = 3
)

type authMerlinResp struct {
	Id    string `json:"localid"`
	Token string `json:"idToken"`
}

type Merlin struct {
	email    string
	password string

	token string

	clipool []*req.Client
}

func NewMerlinIns(email, passwd string) *Merlin {
	// TODO add number
	cli := req.NewClient()

	return &Merlin{
		email:    email,
		password: passwd,
		clipool:  []*req.Client{cli},
	}
}

func (m *Merlin) pickone() (*req.Client, error) {
	if len(m.clipool) > 0 {
		return m.clipool[0], nil
	}
	return nil, fmt.Errorf("no avaliable client")
}

func (m *Merlin) auth() (string, error) {
	var (
		headers = map[string]string{
			"origin": "https://app.getmerlin.in",
		}
		data authMerlinResp
	)
	cli, err := m.pickone()
	if err != nil {
		return "", err
	}
	resp := cli.Post(authurl).SetBody(map[string]interface{}{
		"email":             m.email,
		"password":          m.password,
		"returnSecureToken": true,
	}).SetHeaders(headers).SetResult(&data).Do()

	if resp.Err != nil {
		return "", resp.Err

	}
	if resp.Response.StatusCode/100 != 2 {
		return "", fmt.Errorf("response code is %d", resp.Response.StatusCode)

	}
	return data.Token, nil
}

// 执行请求，若返回401未认证，则重新认证后发送
func (m *Merlin) do(p string) (io.Reader, error) {
	var (
		headers = map[string]string{
			"accept-language": "zh-CN,zh;q=0.9,en",
		}
	)
	if m.token == "" {
		token, err := m.auth()
		if err != nil {
			return nil, err
		}
		m.token = token
	}

}

// 发送文本
func (m *Merlin) SendText(prompt io.Reader) (io.Reader, error) {
	prom, err := io.ReadAll(prompt)
	if err != nil {
		return nil, err
	}
	return m.do(string(prom))
}

// 总额度
func (m *Merlin) Capacity() int {

}

// 可用额度
func (m *Merlin) Allocate() int {

}

// 支持类型
func (m *Merlin) Support() []pkg.PromptType {
	return []pkg.PromptType{pkg.Text}
}

// 支持类型
func (m *Merlin) IsSupport(t pkg.PromptType) bool {
	if t != pkg.Text {
		return false
	}
	return true
}
