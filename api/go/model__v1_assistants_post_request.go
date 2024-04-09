/*
 * OpenAI（ChatGPT）
 *
 * Open AI（ChatGPT）几乎可以应用于任何涉及理解或生成自然语言或代码的任务。我们提供一系列具有不同功率级别的模型，适用于不同的任务，并且能够微调您自己的自定义模型。这些模型可用于从内容生成到语义搜索和分类的所有领域。  
 *
 * API version: 1.0.0
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package openapi

type V1AssistantsPostRequest struct {

	// 要使用的模型的 ID。您可以使用列表模型API 查看所有可用模型，或查看我们的模型概述以获取它们的描述。
	Model string `json:"model"`

	// 助理的名字。最大长度为 256 个字符。
	Name string `json:"name,omitempty"`

	// 助理的描述。最大长度为 512 个字符。
	Description string `json:"description,omitempty"`

	// 助手使用的系统指令。最大长度为 32768 个字符。
	Instructions string `json:"instructions,omitempty"`

	// 助手上启用的工具列表。每个助手最多可以有 128 个工具。工具的类型可以是`code_interpreter`、`retrieval`、 或`function`。
	Tools []V1AssistantsPostRequestToolsInner `json:"tools,omitempty"`

	// 附加到该助手的文件ID列表。助手最多可以附加 20 个文件。文件按其创建日期升序排列。
	FileIds []string `json:"file_ids,omitempty"`

	// 一组可附加到对象的 16 个键值对。这对于以结构化格式存储有关对象的附加信息非常有用。键的最大长度为 64 个字符，值的最大长度为 512 个字符。
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}