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

type AssistantsBetaAPI interface {


    // V1AssistantsAssistantIdDelete Delete /v1/assistants/:assistant_id 
    // 删除助手 
     V1AssistantsAssistantIdDelete(c *gin.Context)

    // V1AssistantsAssistantIdFilesFileIdDelete Delete /v1/assistants/:assistant_id/files/:file_id 
    // 删除辅助文件 
     V1AssistantsAssistantIdFilesFileIdDelete(c *gin.Context)

    // V1AssistantsAssistantIdFilesFileIdGet Get /v1/assistants/:assistant_id/files/:file_id 
    // 检索助手文件 
     V1AssistantsAssistantIdFilesFileIdGet(c *gin.Context)

    // V1AssistantsAssistantIdFilesGet Get /v1/assistants/:assistant_id/files
    // 列出助手文件 
     V1AssistantsAssistantIdFilesGet(c *gin.Context)

    // V1AssistantsAssistantIdFilesPost Post /v1/assistants/:assistant_id/files
    // 创建辅助文件 
     V1AssistantsAssistantIdFilesPost(c *gin.Context)

    // V1AssistantsAssistantIdGet Get /v1/assistants/:assistant_id
    // 检索助手 
     V1AssistantsAssistantIdGet(c *gin.Context)

    // V1AssistantsAssistantIdPost Post /v1/assistants/:assistant_id 
    // 修改助手 
     V1AssistantsAssistantIdPost(c *gin.Context)

    // V1AssistantsGet Get /v1/assistants
    // 列出助手 
     V1AssistantsGet(c *gin.Context)

    // V1AssistantsPost Post /v1/assistants
    // 创建助手 
     V1AssistantsPost(c *gin.Context)

}