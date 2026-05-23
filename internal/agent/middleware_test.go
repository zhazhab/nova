package agent

import (
	"context"
	"strings"
	"testing"
)

// TestHandleUnknownTool 验证 LLM 幻觉调用不存在工具时，处理器返回引导性
// ToolMessage 而不是抛出错误，从而让 Agent 自行修正。
func TestHandleUnknownTool(t *testing.T) {
	result, err := handleUnknownTool(context.Background(), "write_todo", `{"todos":[]}`)
	if err != nil {
		t.Fatalf("处理未知工具不应返回错误: %v", err)
	}
	if !strings.Contains(result, "write_todo") {
		t.Fatalf("结果应包含工具名: %s", result)
	}
	if !strings.Contains(result, "[tool error]") {
		t.Fatalf("结果应携带 [tool error] 前缀以提示模型自我修复: %s", result)
	}
}
