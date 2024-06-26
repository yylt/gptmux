/*
 * OpenAI（ChatGPT）
 *
 * Open AI（ChatGPT）几乎可以应用于任何涉及理解或生成自然语言或代码的任务。我们提供一系列具有不同功率级别的模型，适用于不同的任务，并且能够微调您自己的自定义模型。这些模型可用于从内容生成到语义搜索和分类的所有领域。  
 *
 * API version: 1.0.0
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package openapi

type V1ModerationsPostRequest struct {

	// 要分类的输入文本
	Input string `json:"input"`

	// 有两种内容审核模型可用：`text-moderation-stable`和`text-moderation-latest`。  默认值`text-moderation-latest`将随着时间的推移自动升级。这可确保您始终使用我们最准确的模型。如果您使用`text-moderation-stable`，我们将在更新模型之前提供提前通知。的准确度`text-moderation-stable`可能略低于 的准确度`text-moderation-latest`。
	Model string `json:"model"`
}
