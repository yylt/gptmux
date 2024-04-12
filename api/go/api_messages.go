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

type MessagesAPI interface {


    // V1ThreadsThreadIdMessagesGet Get /v1/threads/:thread_id/messages
    // 列出消息 
     V1ThreadsThreadIdMessagesGet(c *gin.Context)

    // V1ThreadsThreadIdMessagesMessageIdFilesFileIdGet Get /v1 /threads/:thread_id/messages/:message_id/files/:file_id
    // 检索消息文件 
     V1ThreadsThreadIdMessagesMessageIdFilesFileIdGet(c *gin.Context)

    // V1ThreadsThreadIdMessagesMessageIdFilesGet Get /v1/threads/:thread_id/messages/:message_id/files _
    // 列出消息文件 
     V1ThreadsThreadIdMessagesMessageIdFilesGet(c *gin.Context)

    // V1ThreadsThreadIdMessagesMessageIdGet Get /v1/threads/:thread_id/messages/:message_id 
    // 检索消息 
     V1ThreadsThreadIdMessagesMessageIdGet(c *gin.Context)

    // V1ThreadsThreadIdMessagesMessageIdPost Post /v1/threads/:thread_id/messages/:message_id 
    // 修改留言 
     V1ThreadsThreadIdMessagesMessageIdPost(c *gin.Context)

    // V1ThreadsThreadIdMessagesPost Post /v1/threads/:thread_id/messages
    // 创建消息 
     V1ThreadsThreadIdMessagesPost(c *gin.Context)

}