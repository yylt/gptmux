package claude

import (
	"errors"

	"github.com/yylt/gptmux/pkg"
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
	case ping:
		return &pkg.BackResp{
			Err: errors.New("done"),
		}
	default:
	}
	return nil
}
