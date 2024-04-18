package serve

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	openapi "github.com/yylt/gptmux/api/go"
)

type model struct {
	ctx context.Context
}

var (
	now = time.Now().UTC().Unix()
)

func NewModel(ctx context.Context) *model {
	return &model{
		ctx: ctx,
	}
}

// V1ModelsGet Get /v1/models
// 列出模型
func (m *model) V1ModelsGet(c *gin.Context) {
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
func (m *model) V1ModelsModelGet(c *gin.Context) {
	openapi.DefaultHandleFunc(c)
}

// V1ModelsModelidGet Get /v1/models/:modelid
// 检索模型
func (m *model) V1ModelsModelidGet(c *gin.Context) {
	openapi.DefaultHandleFunc(c)
}
