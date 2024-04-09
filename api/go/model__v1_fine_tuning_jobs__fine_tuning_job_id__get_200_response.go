/*
 * OpenAI（ChatGPT）
 *
 * Open AI（ChatGPT）几乎可以应用于任何涉及理解或生成自然语言或代码的任务。我们提供一系列具有不同功率级别的模型，适用于不同的任务，并且能够微调您自己的自定义模型。这些模型可用于从内容生成到语义搜索和分类的所有领域。  
 *
 * API version: 1.0.0
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package openapi

type V1FineTuningJobsFineTuningJobIdGet200Response struct {

	// 对象类型,总是为\"fine_tuning.job\"
	Object string `json:"object"`

	// 对象标识符,可以在API端点中引用
	Id string `json:"id"`

	// 被微调的基础模型
	Model string `json:"model"`

	// 创建微调作业的Unix时间戳(秒)
	CreatedAt int32 `json:"created_at"`

	// 微调作业完成的Unix时间戳(秒)。如果微调作业仍在运行,则值为null
	FinishedAt int32 `json:"finished_at"`

	// 正在创建的微调模型的名称。如果微调作业仍在运行,则值为null
	FineTunedModel string `json:"fine_tuned_model"`

	// 拥有微调作业的组织
	OrganizationId string `json:"organization_id"`

	// 微调作业的编译结果文件ID。可以使用文件API检索结果
	ResultFiles []string `json:"result_files"`

	// 微调作业的当前状态,可以是validating_files、queued、running、succeeded、failed或cancelled
	Status string `json:"status"`

	// 用于训练的文件ID。可以使用文件API检索训练数据
	TrainingFile string `json:"training_file"`

	Hyperparameters V1FineTuningJobsFineTuningJobIdGet200ResponseHyperparameters `json:"hyperparameters"`

	// 此微调作业处理的计费标记总数。如果微调作业仍在运行,则值为null
	TrainedTokens int32 `json:"trained_tokens"`
}