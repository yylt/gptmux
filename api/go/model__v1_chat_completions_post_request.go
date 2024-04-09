/*
 * OpenAI（ChatGPT）
 *
 * Open AI（ChatGPT）几乎可以应用于任何涉及理解或生成自然语言或代码的任务。我们提供一系列具有不同功率级别的模型，适用于不同的任务，并且能够微调您自己的自定义模型。这些模型可用于从内容生成到语义搜索和分类的所有领域。  
 *
 * API version: 1.0.0
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package openapi

type V1ChatCompletionsPostRequest struct {

	// 要使用的模型的 ID。有关哪些模型可与聊天 API 一起使用的详细信息,请参阅模型端点兼容性表。  
	Model string `json:"model"`

	// 至今为止对话所包含的消息列表。Python 代码示例。
	Messages []V1ChatCompletionsPostRequestMessagesInner `json:"messages"`

	// 使用什么采样温度，介于 0 和 2 之间。较高的值（如 0.8）将使输出更加随机，而较低的值（如 0.2）将使输出更加集中和确定。  我们通常建议改变这个或`top_p`但不是两者。
	Temperature int32 `json:"temperature,omitempty"`

	// 一种替代温度采样的方法，称为核采样，其中模型考虑具有 top_p 概率质量的标记的结果。所以 0.1 意味着只考虑构成前 10% 概率质量的标记。  我们通常建议改变这个或`temperature`但不是两者。
	TopP int32 `json:"top_p,omitempty"`

	// 默认为 1 为每个输入消息生成多少个聊天补全选择。
	N int32 `json:"n,omitempty"`

	// 默认为 false 如果设置,则像在 ChatGPT 中一样会发送部分消息增量。标记将以仅数据的服务器发送事件的形式发送,这些事件在可用时,并在 data: [DONE] 消息终止流。Python 代码示例。
	Stream bool `json:"stream,omitempty"`

	// 默认为 null 最多 4 个序列,API 将停止进一步生成标记。
	Stop string `json:"stop,omitempty"`

	// 默认为 inf 在聊天补全中生成的最大标记数。  输入标记和生成标记的总长度受模型的上下文长度限制。计算标记的 Python 代码示例。
	MaxTokens int32 `json:"max_tokens,omitempty"`

	// -2.0 和 2.0 之间的数字。正值会根据到目前为止是否出现在文本中来惩罚新标记，从而增加模型谈论新主题的可能性。  [查看有关频率和存在惩罚的更多信息。](https://platform.openai.com/docs/api-reference/parameter-details)
	PresencePenalty float32 `json:"presence_penalty,omitempty"`

	// 默认为 0 -2.0 到 2.0 之间的数字。正值根据文本目前的存在频率惩罚新标记,降低模型重复相同行的可能性。  有关频率和存在惩罚的更多信息。
	FrequencyPenalty float32 `json:"frequency_penalty,omitempty"`

	// 代表您的最终用户的唯一标识符，可以帮助 OpenAI 监控和检测滥用行为。[了解更多](https://platform.openai.com/docs/guides/safety-best-practices/end-user-ids)。
	User string `json:"user,omitempty"`

	// 指定模型必须输出的格式的对象。  将 { \"type\": \"json_object\" } 启用 JSON 模式,这可以确保模型生成的消息是有效的 JSON。  重要提示:使用 JSON 模式时,还必须通过系统或用户消息指示模型生成 JSON。如果不这样做,模型可能会生成无休止的空白流,直到生成达到令牌限制,从而导致延迟增加和请求“卡住”的外观。另请注意,如果 finish_reason=\"length\",则消息内容可能会被部分切断,这表示生成超过了 max_tokens 或对话超过了最大上下文长度。  显示属性
	ResponseFormat map[string]interface{} `json:"response_format,omitempty"`

	// 此功能处于测试阶段。如果指定,我们的系统将尽最大努力确定性地进行采样,以便使用相同的种子和参数进行重复请求应返回相同的结果。不能保证确定性,您应该参考 system_fingerprint 响应参数来监控后端的更改。
	Seen int32 `json:"seen,omitempty"`

	// 模型可以调用的一组工具列表。目前,只支持作为工具的函数。使用此功能来提供模型可以为之生成 JSON 输入的函数列表。
	Tools []string `json:"tools"`

	// 控制模型调用哪个函数(如果有的话)。none 表示模型不会调用函数,而是生成消息。auto 表示模型可以在生成消息和调用函数之间进行选择。通过 {\"type\": \"function\", \"function\": {\"name\": \"my_function\"}} 强制模型调用该函数。  如果没有函数存在,默认为 none。如果有函数存在,默认为 auto。  显示可能的类型
	ToolChoice map[string]interface{} `json:"tool_choice"`
}
