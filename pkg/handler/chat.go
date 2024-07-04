package handler

import (
	"context"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/schema"
	openapi "github.com/yylt/gptmux/api/go"
	"github.com/yylt/gptmux/mux"
	"github.com/yylt/gptmux/pkg"
	"github.com/yylt/gptmux/pkg/util"
	"k8s.io/klog/v2"
)

var (
	msgType = "message"
)

type chat struct {
	ctx context.Context

	models []mux.Model
}

func NewChat(ctx context.Context, ms ...mux.Model) *chat {
	var (
		models []mux.Model
	)
	for i := range ms {
		if ms[i] == nil {
			continue
		}
		klog.Infof("Add backend '%s', index '%d'", ms[i].Name(), ms[i].Index())
		models = append(models, ms[i])
	}
	sort.Slice(ms, func(i, j int) bool {
		return ms[i].Index() > ms[j].Index()
	})
	return &chat{
		ctx:    ctx,
		models: models,
	}
}

// V1ChatCompletionsPost Post /v1/chat/completions
// 创建聊天补全
func (ca *chat) V1ChatCompletionsPost(c *gin.Context) {
	var (
		body = openapi.V1ChatCompletionsPostRequest{}
	)
	err := c.BindJSON(&body)
	if err != nil {
		c.AbortWithError(http.StatusNotAcceptable, err)
		return
	}
	var (
		opt  = []llms.CallOption{llms.WithModel(body.Model)}
		data = makePrompt(&body)
		ret  = &openapi.V1ChatCompletionsPost200Response{
			Id:      "chatcmpl",
			Object:  "chat.completion",
			Created: int32(time.Now().UTC().Unix()),
		}
	)
	buf := util.GetBuf()
	defer func() {
		klog.V(4).Infof("response data: %s", buf.String())
		util.PutBuf(buf)
	}()
	opt = append(opt, llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
		defer c.Writer.Flush()

		if len(chunk) > 0 {
			ret.Choices = []openapi.V1ChatCompletionsPost200ResponseChoicesInner{
				{
					Delta: openapi.V1ChatCompletionsPost200ResponseChoicesInnerDelta{
						Role:    pkg.RoleAssistant,
						Content: string(chunk),
					},
				},
			}
			buf.Write(chunk)
			c.SSEvent(msgType, ret)
		}
		select {
		case <-c.Writer.CloseNotify():
			c.SSEvent(msgType, "[DONE]")
			return io.EOF
		case <-ctx.Done():
			c.SSEvent(msgType, "[DONE]")
			return io.EOF
		default:
		}

		return nil
	}))

	for _, m := range ca.models {
		_, err = m.GenerateContent(ca.ctx, data, opt...)
		if err == io.EOF || err == nil {
			klog.Infof("model '%s' complete", m.Name())
			return
		} else {
			klog.Warningf("model '%s' generate failed: %v", m.Name(), err)
		}
	}
	if err != nil {
		c.Abort()
	}
}

func makePrompt(req *openapi.V1ChatCompletionsPostRequest) []llms.MessageContent {
	var (
		cont = map[schema.ChatMessageType]llms.MessageContent{}
		kind schema.ChatMessageType
	)
	for _, msg := range req.Messages {
		switch msg.Role {
		case string(schema.ChatMessageTypeAI), pkg.RoleAssistant:
			kind = schema.ChatMessageTypeAI
		case string(schema.ChatMessageTypeHuman), pkg.RoleUser:
			kind = schema.ChatMessageTypeHuman
		case string(schema.ChatMessageTypeSystem):
			kind = schema.ChatMessageTypeSystem
		case string(schema.ChatMessageTypeGeneric):
			kind = schema.ChatMessageTypeGeneric
		}
		v, ok := cont[kind]
		if !ok {
			cont[kind] = llms.MessageContent{
				Role: kind,
				Parts: []llms.ContentPart{
					llms.TextPart(msg.Content),
				},
			}
		} else {
			v.Parts = append(v.Parts, llms.TextPart(msg.Content))
			cont[kind] = v
		}
	}
	ret := make([]llms.MessageContent, 0, len(cont))
	for _, v := range cont {
		ret = append(ret, v)
	}
	klog.V(4).Infof("request body: %s", ret)
	return ret
}
