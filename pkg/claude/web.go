package claude

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	tlsclient "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	"github.com/go-resty/resty/v2"
	"github.com/yylt/gptmux/pkg"
	"github.com/yylt/gptmux/pkg/box"
	"github.com/yylt/gptmux/pkg/util"
	"k8s.io/klog/v2"
)

var (
	ClaudeName = "claude"
	headers    = map[string]string{
		"Origin":          "https://claude.ai",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8,zh-Hans;q=0.7",
		"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	}
)

type Conf struct {
	// chat
	ChatUuid string `yaml:"chat_id,omitempty"`
}

type web struct {
	cli *resty.Client

	chatid string

	orgid string

	auth *auth

	tlscli tlsclient.HttpClient
}

func New(ctx context.Context, cf *Conf, b box.Box) pkg.Backender {
	s := &web{
		chatid: cf.ChatUuid,
	}

	tr, err := util.NewRoundTripper(
		// Reference: https://bogdanfinn.gitbook.io/open-source-oasis/tls-client/client-options
		tlsclient.WithRandomTLSExtensionOrder(), // Chrome 107+
		tlsclient.WithClientProfile(profiles.Chrome_120),
	)
	if err != nil {
		panic(err)
	}
	s.tlscli = tr.Client
	// Set as transport. Don't forget to set the UA!
	s.cli = resty.New().SetTransport(tr).SetHeaders(headers)

	s.auth = newAuth(s, ctx, b)
	ck, err := s.auth.cookie()
	if err != nil {
		panic(err)
	}
	s.orgid, _ = s.user(ck)
	go s.auth.run(time.Hour)

	return s
}

func (c *web) Name() string {
	return ClaudeName
}

func (c *web) Send(prompt string, t pkg.ChatModel) (<-chan *pkg.BackResp, error) {
	cookie, err := c.auth.cookie()
	if err != nil {
		return nil, err
	}

	address := fmt.Sprintf("https://claude.ai/api/organizations/%s/chat_conversations/%s/completion", c.orgid, c.chatid)

	buf := util.GetBuf()
	defer util.PutBuf(buf)
	bs, err := json.Marshal(map[string]string{"prompt": prompt, "timezone": "Asia/Shanghai"})
	if err != nil {
		return nil, err
	}

	req, err := fhttp.NewRequest(http.MethodPost, address, bytes.NewBuffer(bs))
	if err != nil {
		return nil, err
	}
	for _, ck := range cookie {
		req.AddCookie(&fhttp.Cookie{
			Name:  ck.Name,
			Value: ck.Value,
		})
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("content-type", "application/json")
	for k, v := range headers {
		req.Header.Add(k, v)
	}

	resp, err := c.tlscli.Do(req)
	if err != nil {
		return nil, err
	}
	buf.Reset()
	if resp.StatusCode != 200 {
		buf.ReadFrom(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("could not connect: %s, body: %v", http.StatusText(resp.StatusCode), buf.String())
	}
	var rsch = make(chan *pkg.BackResp, 4)
	go func(sch chan *pkg.BackResp, body io.ReadCloser) {
		var (
			respData = &eventResp{}
			err      error
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
				klog.Warningf("not want resp struct, event data")
				continue
			}
			evresp := textProcess(respData)
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

func (c *web) user(hc []*http.Cookie) (org string, err error) {
	if hc == nil {
		return "", fmt.Errorf("cookie must set")
	}
	var u = &user{}
	url := "https://claude.ai/api/auth/current_account"
	resp, err := c.cli.R().SetCookies(hc).SetResult(u).Get(url)
	if err != nil {
		return "", err
	}
	klog.Infof("user request, response: %v", u)
	defer resp.RawBody().Close()
	if util.IsHttp20xCode(resp.StatusCode()) {
		for _, m := range u.Account.Members {
			if m.Org.Name == u.Account.Dname {
				org = m.Org.Uuid
				break
			}
		}
		klog.Infof("orgid is %s", org)
		return org, nil
	}
	return "", fmt.Errorf("failed currentUser, code %d, body: %#v", resp.StatusCode(), resp.Body())
}
