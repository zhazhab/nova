package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"

	"nova/config"
	"nova/internal/book"
)

type LoreEditPlan struct {
	Message string               `json:"message"`
	Ops     []book.LoreOperation `json:"ops"`
}

func GenerateLoreEditPlan(ctx context.Context, cfg *config.Config, instruction string, items []book.LoreItem) (LoreEditPlan, error) {
	if cfg == nil {
		return LoreEditPlan{}, fmt.Errorf("配置不存在")
	}
	instruction = strings.TrimSpace(instruction)
	if instruction == "" {
		return LoreEditPlan{}, fmt.Errorf("资料库编辑指令不能为空")
	}
	temperature := float32(0.1)
	cm, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:      cfg.OpenAIAPIKey,
		Model:       cfg.OpenAIModel,
		BaseURL:     cfg.OpenAIBaseURL,
		Temperature: &temperature,
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
	})
	if err != nil {
		return LoreEditPlan{}, fmt.Errorf("创建资料库编辑模型失败: %w", err)
	}
	itemsJSON, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return LoreEditPlan{}, fmt.Errorf("序列化资料库失败: %w", err)
	}
	userPrompt := fmt.Sprintf("用户编辑指令：\n%s\n\n当前资料库 JSON：\n%s", instruction, string(itemsJSON))
	log.Printf("[lore-editor-agent] generate begin instruction=%s items=%d", promptPartSummary(instruction), len(items))
	msg, err := cm.Generate(ctx, []*schema.Message{
		schema.SystemMessage(loreEditorSystemInstruction()),
		schema.UserMessage(userPrompt),
	})
	if err != nil {
		return LoreEditPlan{}, fmt.Errorf("生成资料库编辑方案失败: %w", err)
	}
	if msg == nil {
		return LoreEditPlan{}, fmt.Errorf("资料库编辑模型返回为空")
	}
	var plan LoreEditPlan
	if err := json.Unmarshal([]byte(strings.TrimSpace(msg.Content)), &plan); err != nil {
		return LoreEditPlan{}, fmt.Errorf("解析资料库编辑方案失败: %w", err)
	}
	if strings.TrimSpace(plan.Message) == "" {
		plan.Message = "资料库 Agent 批量编辑"
	}
	if len(plan.Ops) == 0 {
		return LoreEditPlan{}, fmt.Errorf("资料库编辑方案没有产生任何操作")
	}
	log.Printf("[lore-editor-agent] generate done message=%q ops=%d", plan.Message, len(plan.Ops))
	return plan, nil
}

func loreEditorSystemInstruction() string {
	return strings.TrimSpace(`你是 Nova 的资料库编辑 Agent，负责按照用户指令维护长篇小说资料库。

你只能输出一个 JSON object，不要输出 Markdown、解释、代码块或额外文本。
JSON 格式：
{
  "message": "一句中文变更说明",
  "ops": [
    {
      "op": "create | update | delete",
      "id": "已有资料 ID，update/delete 必填",
      "item": {
        "id": "create 可省略；update 必须与 id 一致",
        "type": "character | world | location | faction | rule | item | other",
        "name": "资料名称",
        "importance": "major | important | minor",
        "tags": ["标签"],
        "content": "Markdown 正文"
      }
    }
  ]
}

规则：
1. 必须使用已有资料的 id 来 update/delete，不要臆造已有资料 ID。
2. update 操作的 item 要给出完整条目字段，不要只给局部字段，避免丢失正文。
3. create 操作要选择准确类型和重要度；不知道类型时用 other，不知道重要度时用 important。
4. content 使用中文 Markdown，保留用户已有设定中仍然有效的信息。
5. 可以一次返回多个操作，以完成用户要求的全资料库整理、合并、改名、补充、删除或一致性修正。
6. 用户要求不明确时，只做低风险整理和补充，不删除资料。`)
}
