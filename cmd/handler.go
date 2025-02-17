package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tmc/langchaingo/llms"
	api "github.com/yylt/gptmux/api/go"
	"github.com/yylt/gptmux/mux"
	"github.com/yylt/gptmux/pkg/util"
	"k8s.io/klog/v2"
)

var (
	msgType = "message"
)

type Controller struct {
	ctx   context.Context
	debug bool
	// v1 completions
	fims []mux.FimModel

	// chat completions
	chats []mux.Model
}

func NewController(ctx context.Context, debug bool, ms ...mux.Model) *Controller {
	var (
		models []mux.Model
	)
	for i := range ms {
		klog.Infof("append upstream '%s', index '%d'", ms[i].Name(), ms[i].Index())
		models = append(models, ms[i])
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].Index() > models[j].Index()
	})
	return &Controller{
		ctx:   ctx,
		debug: debug,
		chats: models,
	}
}

// V1CompletionsPost Post /v1/completions
// 创建完成
func (ca *Controller) V1CompletionsPost(c *gin.Context) {
	var (
		body      = &api.V1CompletionsPostRequest{}
		reterrors []error
	)

	err := c.ShouldBindBodyWithJSON(body)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	if ca.debug {
		klog.Infof("request: %#v", body)
	}
	buf := util.GetBuf()
	defer func() {
		if ca.debug {
			klog.Infof("response: %s", buf.String())
		}
		util.PutBuf(buf)
	}()
	var (
		ret = &api.V1CompletionsPost200Response{
			Id:      "Controllercmpl",
			Object:  "Controller.completion",
			Created: int32(time.Now().UTC().Unix()),
			Model:   body.Model,
		}
		prompt = completionPrompt(body)
		opt    = []llms.CallOption{
			llms.WithTemperature(float64(body.Temperature)),
			llms.WithTopP(float64(body.TopP)),
			llms.WithPresencePenalty(float64(body.PresencePenalty)),
			llms.WithFrequencyPenalty(float64(body.FrequencyPenalty)),
			llms.WithMetadata(map[string]interface{}{mux.ReqBody: body}),
		}
	)

	if body.Stream {
		opt = append(opt, llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			defer func() {
				c.Writer.Header().Add("Content-Type", "text/event-stream")
				c.Writer.Flush()
			}()

			ret.Choices = []api.V1CompletionsPost200ResponseChoicesInner{
				{
					Index:        1,
					Text:         string(chunk),
					FinishReason: "1",
				},
			}
			buf.Write(chunk)
			c.SSEvent(msgType, ret)

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

	for _, m := range ca.chats {
		fm, ok := m.(mux.FimModel)
		if !ok {
			reterrors = append(reterrors, fmt.Errorf("model '%s' not support", m.Name()))
			klog.Warningf("model '%s' not support completion", m.Name())
			continue
		}
		data, err := fm.Completion(ca.ctx, prompt, opt...)
		if err == nil || errors.Is(err, io.EOF) {
			klog.Infof("model '%s' success", m.Name())
			if !body.Stream {
				ret.Choices = []api.V1CompletionsPost200ResponseChoicesInner{
					{
						Text: string(data),
					},
				}
				c.JSON(http.StatusOK, ret)
			}
			break
		} else {
			reterrors = append(reterrors, err)
			klog.Warningf("model '%s' failed: %v", m.Name(), err)
		}
	}
	if len(reterrors) == len(ca.chats) {
		c.AbortWithError(http.StatusInternalServerError, nil)
	}
}

// V1ControllerCompletionsPost Post /v1/chat/completions
// 创建聊天补全
func (ca *Controller) V1ChatCompletionsPost(c *gin.Context) {
	var (
		body         = &api.V1ChatCompletionsPostRequest{}
		rctx, cancle = context.WithCancel(c.Request.Context())
	)
	defer cancle()
	err := c.ShouldBindBodyWithJSON(body)

	if err != nil {
		klog.Errorf("bind json failed: %v, body: %s", err, c.Request.Body)
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	var (
		opt = []llms.CallOption{
			llms.WithTemperature(float64(body.Temperature)),
			llms.WithTopP(float64(body.TopP)),
			llms.WithPresencePenalty(float64(body.PresencePenalty)),
			llms.WithFrequencyPenalty(float64(body.FrequencyPenalty)),
			llms.WithMetadata(map[string]interface{}{mux.ReqBody: body}),
		}

		message = makePrompt(body)
		ret     = &api.V1ChatCompletionsPost200Response{
			Id:      "Controllercmpl",
			Object:  "Controller.completion",
			Created: int32(time.Now().UTC().Unix()),
			Model:   body.Model,
		}
		reterrs []error
	)
	if ca.debug {
		klog.Infof("request body: %q", body)
	}
	buf := util.GetBuf()
	defer func() {
		util.PutBuf(buf)
	}()
	if body.Stream {
		opt = append(opt, llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			defer func() {
				c.Writer.Header().Add("Content-Type", "text/event-stream")
				c.Writer.Flush()
			}()

			ret.Choices = []api.V1ChatCompletionsPost200ResponseChoicesInner{
				{
					Index: 1,
					Delta: api.V1ChatCompletionsPost200ResponseChoicesInnerDelta{
						Role:    mux.RoleAssistant,
						Content: string(chunk),
					},
					FinishReason: "1",
				},
			}
			buf.Write(chunk)
			c.SSEvent(msgType, ret)
			select {
			case <-c.Writer.CloseNotify():
				cancle()
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

	for _, m := range ca.chats {
		data, err := m.GenerateContent(rctx, message, opt...)
		if err == nil || errors.Is(err, io.EOF) {
			klog.Infof("model '%s' success", m.Name())
			if !body.Stream {
				for _, v := range data.Choices {
					buf.WriteString(v.Content)
				}
				ret.Choices = []api.V1ChatCompletionsPost200ResponseChoicesInner{
					{
						//Index: 0,
						Message: api.V1ChatCompletionsPost200ResponseChoicesInnerDelta{
							Content: buf.String(),
						},
					},
				}
				c.JSON(http.StatusOK, ret)
			}
			break
		} else {
			reterrs = append(reterrs, err)
			klog.Warningf("model '%s' failed: %v", m.Name(), err)
		}
	}
	if len(reterrs) == len(ca.chats) {
		c.AbortWithError(http.StatusInternalServerError, nil)
	}
}

// V1ModelsGet Get /v1/models
// 列出模型
func (ca *Controller) V1ModelsGet(c *gin.Context) {
	now := time.Now().UTC().Unix()
	c.JSON(200, api.V1ModelsGet200Response{
		Object: "list",
		Data: []api.V1ModelsGet200ResponseDataInner{
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
func (ca *Controller) V1ModelsModelGet(c *gin.Context) {
	api.DefaultHandleFunc(c)
}

// V1ModelsModelidGet Get /v1/models/:modelid
// 检索模型
func (ca *Controller) V1ModelsModelidGet(c *gin.Context) {

	api.DefaultHandleFunc(c)
}

func completionPrompt(req *api.V1CompletionsPostRequest) string {
	return fmt.Sprintf(`
	<prefix>%s<prefix>
	<suffix>%s<suffix>
	补全<prefix>和<suffix>之间的内容，直接输出补全后的内容，不要包含prefix和suffix
	`, req.Prompt, req.Suffix)
}

func makePrompt(req *api.V1ChatCompletionsPostRequest) []llms.MessageContent {
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
	return ret
}
