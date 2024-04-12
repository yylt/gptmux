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

type RunsAPI interface {


    // V1ThreadsRunsPost Post /v1/threads/runs
    // 创建线程并运行 
     V1ThreadsRunsPost(c *gin.Context)

    // V1ThreadsThreadIdRunsGet Get /v1/threads/:thread_id/runs
    // 列表运行 
     V1ThreadsThreadIdRunsGet(c *gin.Context)

    // V1ThreadsThreadIdRunsPost Post /v1/threads/:thread_id/runs
    // 创建运行 
     V1ThreadsThreadIdRunsPost(c *gin.Context)

    // V1ThreadsThreadIdRunsRunIdCancelPost Post /v1/threads/:thread_id/runs/:run_id/cancel
    // 取消运行 
     V1ThreadsThreadIdRunsRunIdCancelPost(c *gin.Context)

    // V1ThreadsThreadIdRunsRunIdGet Get /v1/threads/:thread_id/runs/:run_id 
    // 修改运行 
     V1ThreadsThreadIdRunsRunIdGet(c *gin.Context)

    // V1ThreadsThreadIdRunsRunIdStepsGet Get /v1/threads/:thread_id/runs/:run_id/steps
    // 列出运行步骤 
     V1ThreadsThreadIdRunsRunIdStepsGet(c *gin.Context)

    // V1ThreadsThreadIdRunsRunIdStepsStepIdGet Get /v1 /threads/:thread_id/runs/:run_id/steps/:step_id
    // 检索运行步骤 
     V1ThreadsThreadIdRunsRunIdStepsStepIdGet(c *gin.Context)

    // V1ThreadsThreadIdRunsRunIdSubmitToolOutputsPost Post /v1/threads/:thread_id/runs/:run_id/submit_tool_outputs
    // 提交工具输出以运行 
     V1ThreadsThreadIdRunsRunIdSubmitToolOutputsPost(c *gin.Context)

}