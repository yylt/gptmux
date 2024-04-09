/*
 * OpenAI（ChatGPT）
 *
 * Open AI（ChatGPT）几乎可以应用于任何涉及理解或生成自然语言或代码的任务。我们提供一系列具有不同功率级别的模型，适用于不同的任务，并且能够微调您自己的自定义模型。这些模型可用于从内容生成到语义搜索和分类的所有领域。  
 *
 * API version: 1.0.0
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package openapi

type V1FineTuningJobsGet200Response struct {

	// 对象类型,总是为\"fine_tuning.job\"
	Object string `json:"object"`

	Data []V1FineTuningJobsGet200ResponseDataInner `json:"data"`

	HasMore bool `json:"has_more"`
}
