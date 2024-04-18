package claude

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/yylt/gptmux/pkg"
	"github.com/yylt/gptmux/pkg/util"
	"k8s.io/klog/v2"
)

const (
	chunk = "completion"
	ping  = "ping"
)

type user struct {
	Account struct {
		Fname   string `json:"full_name"`
		Dname   string `json:"display_name"`
		Members []struct {
			Org struct {
				Uuid string `json:"uuid"`
				Name string `json:"name"`
			} `json:"organization"`
		} `json:"memberships"`
	} `json:"account"`
}

type eventResp struct {
	Content string `json:"completion,omitempty"`
	Type    string `json:"type"`
	Model   string `json:"model,omitempty"`
	Stop    string `json:"stop,omitempty"`
}

func textProcess(er *eventResp) *pkg.BackResp {
	if er == nil {
		return nil
	}
	switch er.Type {
	case chunk:
		if len(er.Stop) > 0 {
			return &pkg.BackResp{
				Err: errors.New("done"),
			}
		}
		return &pkg.BackResp{
			Content: er.Content,
		}
	default:
	}
	return nil
}

func text(er *eventResp) (content string, done bool) {
	if er == nil {
		return "", false
	}
	switch er.Type {
	case chunk:
		if len(er.Stop) > 0 {
			done = true
		}
		content = er.Content
	default:
	}
	return
}

func process(body io.ReadCloser, fn func(*eventResp) error) {
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
			klog.Warningf("invalid data struct: %v", err)
			continue
		}
		err = fn(respData)
		if err != nil {
			klog.Warningf("callback error %s, abort", err)
			return
		}
	}
}
