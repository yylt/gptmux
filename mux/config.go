package mux

import (
	"unicode/utf8"

	"github.com/tmc/langchaingo/llms"
	"github.com/yylt/gptmux/pkg/util"
)

type ChatModel string

const (
	RoleAssistant = "assistant"
	RoleUser      = "user"
	RoleSystem    = "system"

	NonModel ChatModel = ""
	ImgModel ChatModel = "image"
	TxtModel ChatModel = "text"
)

var (
	hua, _ = utf8.DecodeRuneInString("画")
)

type Model interface {
	llms.Model

	Name() string
	Index() int
}

// system and the last human
func GeneraPrompt(messages []llms.MessageContent) (string, ChatModel) {
	var (
		m           = TxtModel
		buf         = util.GetBuf()
		txt, prefix llms.TextContent

		iszh bool
	)
	defer util.PutBuf(buf)
	for _, msg := range messages {
		switch msg.Role {
		case llms.ChatMessageTypeHuman:
			leng := len(msg.Parts)
			if leng < 1 {
				return "", NonModel
			}
			txt = msg.Parts[leng-1].(llms.TextContent)
			first, _ := utf8.DecodeRuneInString(txt.Text)
			if first == hua {
				m = ImgModel
			} else {
				if util.HasChineseChar(txt.Text) {
					iszh = true
				}
			}

		case llms.ChatMessageTypeSystem:
			leng := len(msg.Parts)
			if leng < 1 {
				continue
			}
			prefix = msg.Parts[leng-1].(llms.TextContent)
			if util.HasChineseChar(prefix.Text) {
				iszh = true
			}
		default:
		}
	}
	if txt.Text == "" {
		return "", NonModel
	}

	if prefix.Text != "" {
		buf.WriteString(prefix.Text + "\n")
	}

	if m == TxtModel && !iszh {
		if !util.HasChineseChar(txt.Text) {
			buf.WriteString("使用中文, ")
		}
	}
	buf.WriteString(txt.Text)

	return buf.String(), m
}
