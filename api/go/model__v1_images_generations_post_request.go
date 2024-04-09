/*
 * OpenAI（ChatGPT）
 *
 * Open AI（ChatGPT）几乎可以应用于任何涉及理解或生成自然语言或代码的任务。我们提供一系列具有不同功率级别的模型，适用于不同的任务，并且能够微调您自己的自定义模型。这些模型可用于从内容生成到语义搜索和分类的所有领域。  
 *
 * API version: 1.0.0
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package openapi

type V1ImagesGenerationsPostRequest struct {

	// 所需图像的文本描述。最大长度为 1000 个字符。
	Prompt string `json:"prompt"`

	// 用于图像生成的模型。
	Model string `json:"model,omitempty"`

	// 要生成的图像数。必须介于 1 和 10 之间。
	N int32 `json:"n,omitempty"`

	// 将生成的图像的质量。`hd`创建具有更精细细节和更高一致性的图像。此参数仅支持`dall-e-3`.
	Quality string `json:"quality,omitempty"`

	// 返回生成的图像的格式。必须是 或url之一b64_json。
	ResponseFormat string `json:"response_format,omitempty"`

	// 生成图像的大小。必须是`256x256`、`512x512`或`1024x1024`for之一`dall-e-2`。对于模型来说，必须是`1024x1024`、`1792x1024`、 或之一。`1024x1792``dall-e-3`
	Style string `json:"style,omitempty"`

	// 生成图像的风格。必须是 或`vivid`之一`natural`。生动使模型倾向于生成超真实和戏剧性的图像。自然使模型生成更自然、不太真实的图像。此参数仅支持`dall-e-3`.
	User string `json:"user,omitempty"`

	// 生成图像的大小。必须是256x256、512x512或 1024x1024之一。
	Size string `json:"size,omitempty"`
}