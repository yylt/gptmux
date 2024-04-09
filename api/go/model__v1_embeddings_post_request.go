/*
 * OpenAI（ChatGPT）
 *
 * Open AI（ChatGPT）几乎可以应用于任何涉及理解或生成自然语言或代码的任务。我们提供一系列具有不同功率级别的模型，适用于不同的任务，并且能够微调您自己的自定义模型。这些模型可用于从内容生成到语义搜索和分类的所有领域。  
 *
 * API version: 1.0.0
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package openapi

type V1EmbeddingsPostRequest struct {

	// 要使用的模型的 ID。您可以使用[List models](https://platform.openai.com/docs/api-reference/models/list) API 来查看所有可用模型，或查看我们的[模型概述](https://platform.openai.com/docs/models/overview)以了解它们的描述。
	Model string `json:"model"`

	// 输入文本以获取嵌入，编码为字符串或标记数组。要在单个请求中获取多个输入的嵌入，请传递一个字符串数组或令牌数组数组。每个输入的长度不得超过 8192 个标记。
	Input string `json:"input"`
}