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

type FilesAPI interface {


    // V1FilesFileIdContentGet Get /v1/files/:file_id/content
    // 检索文件内容 
     V1FilesFileIdContentGet(c *gin.Context)

    // V1FilesFileIdDelete Delete /v1/files/:file_id
    // 删除文件 
     V1FilesFileIdDelete(c *gin.Context)

    // V1FilesFileIdGet Get /v1/files/:file_id
    // 检索文件 
     V1FilesFileIdGet(c *gin.Context)

    // V1FilesGet Get /v1/files _
    // 列出文件 
     V1FilesGet(c *gin.Context)

    // V1FilesPost Post /v1/files
    // 上传文件 
     V1FilesPost(c *gin.Context)

}