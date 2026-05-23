package agent

import (
	"context"
	"fmt"
	"log"
	"strings"

	localbk "github.com/cloudwego/eino-ext/adk/backend/local"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/skill"
	"github.com/cloudwego/eino/adk/prebuilt/deep"
	"github.com/cloudwego/eino/compose"

	"nova/config"
	"nova/internal/book"
)

// Build 构建小说创作 Agent（deep agent + 文件系统工具 + Skill 中间件）。
func Build(ctx context.Context, cfg *config.Config, state *book.State) (adk.Agent, error) {
	cm, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  cfg.OpenAIAPIKey,
		Model:   cfg.OpenAIModel,
		BaseURL: cfg.OpenAIBaseURL,
	})
	if err != nil {
		return nil, fmt.Errorf("创建模型失败: %w", err)
	}

	backend, err := localbk.NewBackend(ctx, &localbk.Config{})
	if err != nil {
		return nil, fmt.Errorf("创建 backend 失败: %w", err)
	}

	var handlers []adk.ChatModelAgentMiddleware
	if skillsDir := ResolveSkillsDir(cfg.SkillsDir); skillsDir != "" {
		skillBackend, sbErr := skill.NewBackendFromFilesystem(ctx, &skill.BackendFromFilesystemConfig{
			Backend: backend,
			BaseDir: skillsDir,
		})
		if sbErr == nil {
			skillMw, smErr := skill.NewMiddleware(ctx, &skill.Config{Backend: skillBackend})
			if smErr == nil {
				handlers = append(handlers, skillMw)
			}
		}
	}
	handlers = append(handlers, &safeToolMiddleware{})

	return deep.New(ctx, &deep.Config{
		Name:           "NovaAgent",
		Description:    "AI 小说创作助手",
		ChatModel:      cm,
		Instruction:    BuildInstruction(cfg, state),
		Backend:        backend,
		StreamingShell: backend,
		MaxIteration:   50,
		Handlers:       handlers,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				// 当 LLM 幻觉出不存在的工具时，把错误信息以 ToolMessage 形式回传，
				// 让 Agent 在下一轮自行修正工具名或改用其他方案，避免整次任务被 NodeRunError 中断。
				UnknownToolsHandler: handleUnknownTool,
			},
		},
		ModelRetryConfig: &adk.ModelRetryConfig{
			MaxRetries: 5,
			IsRetryAble: func(_ context.Context, err error) bool {
				return strings.Contains(err.Error(), "429") ||
					strings.Contains(err.Error(), "Too Many Requests") ||
					strings.Contains(err.Error(), "qpm limit")
			},
		},
	})
}

// handleUnknownTool 拦截 LLM 调用未知工具的错误，把可读提示作为工具结果回传给模型，
// 引导 Agent 在后续轮次基于该反馈自我修正（例如改用正确的工具名）。
func handleUnknownTool(_ context.Context, name, input string) (string, error) {
	log.Printf("[agent] LLM 调用了不存在的工具 name=%s args=%s", name, input)
	return fmt.Sprintf(
		"[tool error] 工具 %q 不存在或当前不可用。请基于该错误自我分析：\n"+
			"1) 如果是工具名拼写错误（例如 write_todo 应为 write_todos），请在下一步使用正确的工具名重新调用；\n"+
			"2) 如果该能力无法通过现有工具完成，请改用其他可用工具或直接以文本回复用户；\n"+
			"3) 不要重复调用同一个不存在的工具。",
		name,
	), nil
}
