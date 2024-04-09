/*
 * OpenAI（ChatGPT）
 *
 * Open AI（ChatGPT）几乎可以应用于任何涉及理解或生成自然语言或代码的任务。我们提供一系列具有不同功率级别的模型，适用于不同的任务，并且能够微调您自己的自定义模型。这些模型可用于从内容生成到语义搜索和分类的所有领域。  
 *
 * API version: 1.0.0
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package openapi

type V1ThreadsThreadIdRunsPostRequestToolsInnerFunctionToolFunction struct {

	// 对函数功能的描述，模型使用它来选择何时以及如何调用该函数。
	Description string `json:"description,omitempty"`

	// 要调用的函数的名称。必须是 az、AZ、0-9，或包含下划线和破折号，最大长度为 64。
	Name string `json:"name"`

	// 函数接受的参数，描述为 JSON Schema 对象。请参阅指南以获取示例，并参阅 JSON 架构参考以获取有关格式的文档。  要描述不接受参数的函数，请提供值{\"type\": \"object\", \"properties\": {}}。
	Parameters map[string]interface{} `json:"parameters"`
}
