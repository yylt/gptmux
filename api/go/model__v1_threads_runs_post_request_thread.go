/*
 * OpenAI（ChatGPT）
 *
 * Open AI（ChatGPT）几乎可以应用于任何涉及理解或生成自然语言或代码的任务。我们提供一系列具有不同功率级别的模型，适用于不同的任务，并且能够微调您自己的自定义模型。这些模型可用于从内容生成到语义搜索和分类的所有领域。  
 *
 * API version: 1.0.0
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package openapi

type V1ThreadsRunsPostRequestThread struct {

	// 用于启动线程的消息列表。
	Messages []V1ThreadsRunsPostRequestThreadMessagesInner `json:"messages,omitempty"`

	// 一组可附加到对象的 16 个键值对。这对于以结构化格式存储有关对象的附加信息非常有用。键的最大长度为 64 个字符，值的最大长度为 512 个字符。
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}
