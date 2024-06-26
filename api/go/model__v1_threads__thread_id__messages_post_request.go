/*
 * OpenAI（ChatGPT）
 *
 * Open AI（ChatGPT）几乎可以应用于任何涉及理解或生成自然语言或代码的任务。我们提供一系列具有不同功率级别的模型，适用于不同的任务，并且能够微调您自己的自定义模型。这些模型可用于从内容生成到语义搜索和分类的所有领域。  
 *
 * API version: 1.0.0
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package openapi

type V1ThreadsThreadIdMessagesPostRequest struct {

	// 创建消息的实体的角色。目前仅user支持。
	Role string `json:"role"`

	// 消息的内容。
	Content string `json:"content"`

	// 消息应使用的文件ID列表。一条消息最多可以附加 10 个文件。retrieval对于code_interpreter可以访问和使用文件的工具非常有用。
	FileIds []string `json:"file_ids,omitempty"`

	// 一组可附加到对象的 16 个键值对。这对于以结构化格式存储有关对象的附加信息非常有用。键的最大长度为 64 个字符，值的最大长度为 512 个字符。
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}
