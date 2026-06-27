package support

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QB-Chen/wcfLink/internal/llm"
)

const builderSystemPrompt = `你是客服规范配置助手。你的任务是通过与用户的多轮对话，收集信息并生成一套完整的客服规范配置。

## 你需要收集的信息

### 必须收集（缺一不可）：
1. **配置名称** (name)：用于识别这套配置，如 "电商客服" / "SaaS技术支持"
2. **公司/品牌名称** (company_name)：客服代表的公司
3. **行业类型** (industry)：如电商、SaaS、教育、金融等

### 建议收集（有默认值）：
4. **问候语** (greeting)：客服开场白
5. **退款政策** (refund_policy)：退款条件和限额
6. **升级阈值** (escalation_threshold)：超过此金额的退款需要人工审核（默认 500）
7. **营业时间** (business_hours)：如 "周一到周五 9:00-18:00"
8. **特殊规则** (extra_rules)：任何额外的业务规则

## 对话流程

1. 先问用户要配置什么类型的客服规范
2. 逐步收集上述信息，每次只问 1-2 个问题
3. 用户回答后确认理解是否正确
4. 所有必须信息收集完毕后，生成完整配置
5. 展示给用户确认，确认后输出 JSON

## 输出格式

当所有信息收集完毕且用户确认后，你必须输出如下格式的 JSON（用 ` + "```json" + ` 和 ` + "```" + ` 包裹）：

` + "```json" + `
{
  "action": "create_profile",
  "profile": {
    "name": "配置名称",
    "company_name": "公司名",
    "industry": "行业",
    "greeting": "问候语",
    "escalation_threshold": 500,
    "refund_policy": "退款政策描述",
    "business_hours": "营业时间",
    "extra_config": "额外规则",
    "is_default": false
  }
}
` + "```" + `

## 重要规则
- 在信息不完整时不要生成 JSON，继续追问
- 保持友好引导的语气
- 如果用户说"设为默认"，则 is_default 为 true
- 生成的配置会被用于自定义客服模式的系统提示词`

type Builder struct {
	llmClient *llm.Client
	store     *Store
}

func NewBuilder(llmClient *llm.Client, store *Store) *Builder {
	return &Builder{
		llmClient: llmClient,
		store:     store,
	}
}

type BuilderAction struct {
	Action  string  `json:"action"`
	Profile Profile `json:"profile"`
}

func (b *Builder) ProcessSetupMessage(ctx context.Context, history []llm.Message, userMessage string, temperature *float64, maxTokens int) (string, *Profile, error) {
	messages := make([]llm.Message, 0, len(history)+2)
	messages = append(messages, llm.Message{Role: llm.RoleSystem, Content: builderSystemPrompt})
	messages = append(messages, history...)
	messages = append(messages, llm.Message{Role: llm.RoleUser, Content: userMessage})

	resp, err := b.llmClient.Chat(ctx, llm.ChatRequest{
		Messages:    messages,
		Temperature: temperature,
		MaxTokens:   maxTokens,
	})
	if err != nil {
		return "", nil, fmt.Errorf("builder LLM call failed: %w", err)
	}

	reply := resp.Choices[0].Message.Content

	profile := extractProfileJSON(reply)
	if profile != nil {
		created, err := b.store.ProfileCreate(ctx, *profile)
		if err != nil {
			return reply + "\n\n（保存配置失败: " + err.Error() + "）", nil, nil
		}
		cleanReply := removeJSONBlock(reply)
		confirmMsg := fmt.Sprintf("%s\n\n客服规范「%s」已创建成功！（ID: %s）", cleanReply, created.Name, created.ID)
		if created.IsDefault {
			confirmMsg += "\n已设为默认客服规范。"
		}
		confirmMsg += "\n\n使用 /support 切换到客服模式即可生效。"
		return confirmMsg, &created, nil
	}

	return reply, nil, nil
}

func extractProfileJSON(text string) *Profile {
	start := strings.Index(text, "```json")
	if start == -1 {
		return nil
	}
	start += len("```json")
	end := strings.Index(text[start:], "```")
	if end == -1 {
		return nil
	}
	jsonStr := strings.TrimSpace(text[start : start+end])

	var action BuilderAction
	if err := json.Unmarshal([]byte(jsonStr), &action); err != nil {
		return nil
	}
	if action.Action != "create_profile" {
		return nil
	}
	if action.Profile.Name == "" {
		return nil
	}
	return &action.Profile
}

func removeJSONBlock(text string) string {
	start := strings.Index(text, "```json")
	if start == -1 {
		return text
	}
	end := strings.Index(text[start+len("```json"):], "```")
	if end == -1 {
		return text[:start]
	}
	endPos := start + len("```json") + end + len("```")
	return strings.TrimSpace(text[:start] + text[endPos:])
}

func GenerateCustomPrompt(base string, profile Profile) string {
	var sb strings.Builder
	sb.WriteString(base)
	sb.WriteString("\n\n# 自定义客服配置\n")

	if profile.CompanyName != "" {
		sb.WriteString(fmt.Sprintf("\n你代表的公司/品牌：%s\n", profile.CompanyName))
	}
	if profile.Industry != "" {
		sb.WriteString(fmt.Sprintf("行业：%s\n", profile.Industry))
	}
	if profile.Greeting != "" {
		sb.WriteString(fmt.Sprintf("\n## 开场问候语\n%s\n", profile.Greeting))
	}
	if profile.RefundPolicy != "" {
		sb.WriteString(fmt.Sprintf("\n## 退款政策\n%s\n", profile.RefundPolicy))
	}
	if profile.EscalationThreshold > 0 {
		sb.WriteString(fmt.Sprintf("\n## 升级阈值\n退款金额超过 %.0f 元必须升级处理。\n", profile.EscalationThreshold))
	}
	if profile.BusinessHours != "" {
		sb.WriteString(fmt.Sprintf("\n## 营业时间\n%s\n非营业时间收到的消息，告知客户会在营业时间内处理。\n", profile.BusinessHours))
	}
	if profile.ExtraConfig != "" {
		sb.WriteString(fmt.Sprintf("\n## 额外规则\n%s\n", profile.ExtraConfig))
	}

	return sb.String()
}
