package serve

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/schema"
	openapi "github.com/yylt/gptmux/api/go"
	"github.com/yylt/gptmux/mux"
	"github.com/yylt/gptmux/pkg"
	"k8s.io/klog/v2"
)

type chat struct {
	ctx context.Context

	models []mux.Model
}

func New(ms ...mux.Model) *chat {
	return &chat{
		models: append([]mux.Model{}, ms...),
	}
}

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
		opt  []llms.CallOption
		data = makePrompt(&body)
	)

	if strings.Contains(c.GetHeader("Accept"), "text/event-stream") || body.Stream {
		var (
			ret = &openapi.V1ChatCompletionsPost200Response{
				Id:     body.Model,
				Object: body.Model,
			}
		)
		opt = append(opt, llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			select {
			case <-ctx.Done():
				ret.Choices = []openapi.V1ChatCompletionsPost200ResponseChoicesInner{
					{
						FinishReason: "stop",
					},
				}
				c.SSEvent("message", ret)
				return fmt.Errorf("done")
			default:
				ret.Choices = []openapi.V1ChatCompletionsPost200ResponseChoicesInner{
					{
						Message: openapi.V1ChatCompletionsPost200ResponseChoicesInnerMessage{
							Role:    pkg.RoleAssistant,
							Content: string(chunk),
						},
					},
				}
				c.SSEvent("message", ret)
			}

			return nil
		}))
	} else {
		c.AbortWithError(http.StatusNotAcceptable, fmt.Errorf("only support SSE"))
		return
	}

	for _, m := range ca.models {
		_, err = m.GenerateContent(ca.ctx, data, opt...)
		if err != nil {
			klog.Warningf("model '%s' generate failed: %v", m.Name(), err)
		}
	}

}

func makePrompt(req *openapi.V1ChatCompletionsPostRequest) []llms.MessageContent {
	var (
		cont = make([]llms.MessageContent, len(req.Messages))
	)
	for _, msg := range req.Messages {
		llmmsg := llms.MessageContent{
			Role: schema.ChatMessageType(msg.Role),
		}
		llmmsg.Parts=

	}
	return nil
}
