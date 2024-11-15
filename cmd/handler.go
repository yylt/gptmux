package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tmc/langchaingo/llms"
	openapi "github.com/yylt/gptmux/api/go"
	"github.com/yylt/gptmux/mux"
	"github.com/yylt/gptmux/pkg/util"
	"k8s.io/klog/v2"
)

var (
	msgType = "message"
	now     = time.Now().UTC().Unix()
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
		klog.Infof("append backend '%s', index '%d'", ms[i].Name(), ms[i].Index())
		models = append(models, ms[i])
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].Index() > models[j].Index()
	})
	return &chat{
		ctx:    ctx,
		models: models,
	}
}

// V1CompletionsPost Post /v1/completions
// 创建完成
func (ca *chat) V1CompletionsPost(c *gin.Context) {
	openapi.DefaultHandleFunc(c)
}

// V1ChatCompletionsPost Post /v1/chat/completions
// 创建聊天补全
func (ca *chat) V1ChatCompletionsPost(c *gin.Context) {
	var (
		body = openapi.V1ChatCompletionsPostRequest{}
	)
	err := c.ShouldBindJSON(&body)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	req, _ := json.Marshal(body)
	klog.V(3).Infof("stream: %t, request data: %s", body.Stream, string(req))
	var (
		opt  = []llms.CallOption{}
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
	if body.Stream {
		opt = append(opt, llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			defer func() {
				c.Writer.Header().Add("Content-Type", "text/event-stream")
				c.Writer.Flush()
			}()
			if len(chunk) > 0 {
				ret.Choices = []openapi.V1ChatCompletionsPost200ResponseChoicesInner{
					{
						Delta: openapi.V1ChatCompletionsPost200ResponseChoicesInnerDelta{
							Role:    mux.RoleAssistant,
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
	}

	for _, m := range ca.models {
		data, err := m.GenerateContent(ca.ctx, data, opt...)
		if err == io.EOF || err == nil {
			klog.V(3).Infof("model '%s' success", m.Name())
			if !body.Stream {
				for _, v := range data.Choices {
					buf.WriteString(v.Content)
				}
				ret.Choices = []openapi.V1ChatCompletionsPost200ResponseChoicesInner{
					{
						Message: openapi.V1ChatCompletionsPost200ResponseChoicesInnerDelta{
							Content: buf.String(),
						},
					},
				}
				req, _ := json.Marshal(ret)
				klog.V(3).Infof("response data: %s", string(req))
				c.JSON(http.StatusOK, ret)
			}
			break
		} else {
			klog.Warningf("model '%s' failed: %v", m.Name(), err)
		}
	}
}

// V1ModelsGet Get /v1/models
// 列出模型
func (ca *chat) V1ModelsGet(c *gin.Context) {
	c.JSON(200, openapi.V1ModelsGet200Response{
		Object: "list",
		Data: []openapi.V1ModelsGet200ResponseDataInner{
			{
				Id:      "gpt-3.5-turbo",
				Object:  "object",
				Created: int32(now),
				OwnedBy: "openai",
			},
			{
				Id:      "gpt-4-turbo",
				Object:  "object",
				Created: int32(now),
				OwnedBy: "openai",
			},
		},
	})
}

// V1ModelsModelGet Get /v1/models/:model
// 删除微调模型
func (ca *chat) V1ModelsModelGet(c *gin.Context) {
	openapi.DefaultHandleFunc(c)
}

// V1ModelsModelidGet Get /v1/models/:modelid
// 检索模型
func (ca *chat) V1ModelsModelidGet(c *gin.Context) {
	openapi.DefaultHandleFunc(c)
}

func makePrompt(req *openapi.V1ChatCompletionsPostRequest) []llms.MessageContent {
	var (
		cont = map[llms.ChatMessageType]llms.MessageContent{}
		kind llms.ChatMessageType
	)
	for _, msg := range req.Messages {
		switch msg.Role {
		case string(llms.ChatMessageTypeAI), mux.RoleAssistant:
			kind = llms.ChatMessageTypeAI
		case string(llms.ChatMessageTypeHuman), mux.RoleUser:
			kind = llms.ChatMessageTypeHuman
		case string(llms.ChatMessageTypeSystem):
			kind = llms.ChatMessageTypeSystem
		case string(llms.ChatMessageTypeGeneric):
			kind = llms.ChatMessageTypeGeneric
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
	klog.V(5).Infof("request body: %s", ret)
	return ret
}
