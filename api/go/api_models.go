/*
 * OpenAI（ChatGPT）
 *
 * Open AI（ChatGPT）几乎可以应用于任何涉及理解或生成自然语言或代码的任务。我们提供一系列具有不同功率级别的模型，适用于不同的任务，并且能够微调您自己的自定义模型。这些模型可用于从内容生成到语义搜索和分类的所有领域。  
 *
 * API version: 1.0.0
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package openapi

import (
	"github.com/gin-gonic/gin"
)

type ModelsAPI interface {


    // V1ModelsGet Get /v1/models
    // 列出模型 
     V1ModelsGet(c *gin.Context)

    // V1ModelsModelGet Get /v1/models/:model
    // 删除微调模型 
     V1ModelsModelGet(c *gin.Context)

    // V1ModelsModelidGet Get /v1/models/:modelid
    // 检索模型 
     V1ModelsModelidGet(c *gin.Context)

}