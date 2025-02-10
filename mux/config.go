package mux

import (
	"context"
	"unicode/utf8"

	"github.com/tmc/langchaingo/llms"
	"github.com/yylt/gptmux/pkg/util"
)

type ChatModel string

const (
	RoleAssistant = "assistant"

	RoleUser   = "user"
	RoleSystem = "system"

	NonModel ChatModel = ""
	ImgModel ChatModel = "image"
	TxtModel ChatModel = "text"

	ReqBody = "req"
)

var (
	hua, _ = utf8.DecodeRuneInString("画")
)

type Model interface {
	llms.Model

	Name() string
	Index() int
}
type FimModel interface {
	Completion(ctx context.Context, prompt string, options ...llms.CallOption) (string, error)
}

// system and the last human
func GeneraPrompt(messages []llms.MessageContent) (string, ChatModel) {
	var (
		m           = TxtModel
		buf         = util.GetBuf()
		txt, prefix llms.TextContent
		iszh        bool
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
		buf.WriteString(prefix.Text + "\r\n")
	}
	buf.WriteString(txt.Text)
	if m == TxtModel && !iszh {
		if !util.HasChineseChar(txt.Text) {
			buf.WriteString("\r\n请使用中文回答")
		}
	}

	return buf.String(), m
}

// last system and the last human
func NormalPrompt(messages []llms.MessageContent) []llms.MessageContent {
	var (
		ret []llms.MessageContent
	)

	for _, msg := range messages {
		switch msg.Role {
		case llms.ChatMessageTypeHuman:
			leng := len(msg.Parts)
			if leng < 1 {
				return nil
			}
			ret = append(ret, llms.MessageContent{
				Role:  llms.ChatMessageTypeHuman,
				Parts: []llms.ContentPart{msg.Parts[leng-1].(llms.TextContent)},
			})

		case llms.ChatMessageTypeSystem:
			leng := len(msg.Parts)
			if leng < 1 {
				continue
			}
			ret = append(ret, llms.MessageContent{
				Role:  llms.ChatMessageTypeHuman,
				Parts: []llms.ContentPart{msg.Parts[leng-1]},
			})
		}
	}
	ret = append(ret, llms.MessageContent{
		Role: llms.ChatMessageTypeSystem,
		Parts: []llms.ContentPart{llms.TextContent{
			Text: "请使用中文回答",
		}}})
	return ret
}

// last system and the last human
func CompletionPrompt(prompt string) []llms.MessageContent {
	ret := []llms.MessageContent{
		{
			Role: llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{llms.TextContent{
				Text: prompt,
			}},
		},
	}
	return ret
}
