package box

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/yylt/gptmux/pkg/util"
)

const (
	tangurl = "https://sctapi.ftqq.com/%s.send"
)

var _ Notifyer = &tang{}

type tang struct {
	token string `yaml:"token,omitempty"`
}

func newTang(t string) *tang {
	if t == "" {
		return nil
	}
	return &tang{
		token: t,
	}
}
func (t *tang) Name() string {
	return "tang"
}
func (t *tang) Push(msg *Message) error {

	data := url.Values{}
	data.Set("text", msg.Title)
	data.Set("desp", msg.Msg)

	client := &http.Client{}
	req, err := http.NewRequest("POST", fmt.Sprintf(tangurl, t.token), strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if util.IsHttp20xCode(resp.StatusCode) {
		return nil
	}
	return fmt.Errorf("failed send fangTang, code %d", resp.StatusCode)
}
