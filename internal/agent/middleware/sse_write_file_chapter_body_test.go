package middleware

import (
	"strings"
	"testing"

	"nova/internal/agent"
)

func TestSSEWriteFileChapterBodyMiddlewareShowsOnlyPathForToolCall(t *testing.T) {
	collector, handler := newWriteFileChapterBodySSETestHandler()
	args := `{"file_path":"chapters/ch01.md","content":"第一行\n第二行"}`

	got := mustForwardSSEEvent(t, collector, handler, agent.Event{Type: "tool_call", Data: map[string]interface{}{
		"agent_kind": agent.AgentKindIDE,
		"id":         "call-1",
		"name":       "write_file",
		"args":       args,
	}})

	gotArgs := eventDataString(got.Data, "args")
	if gotArgs != `{"file_path":"chapters/ch01.md"}` {
		t.Fatalf("display args should keep only path, got %q", gotArgs)
	}
	if strings.Contains(gotArgs, "第一行") || strings.Contains(gotArgs, "content") || strings.Contains(gotArgs, "...") {
		t.Fatalf("display args should not include body or placeholder, got %q", gotArgs)
	}
	assertChapterBodyHiddenNotice(t, got.Data)
}

func TestSSEWriteFileChapterBodyMiddlewareShowsOnlyPathForAbsoluteNovaChapterToolCall(t *testing.T) {
	collector, handler := newWriteFileChapterBodySSETestHandler()
	path := `/Users/huangyongquan/.codex/worktrees/999d/nova/.nova/测试/chapters/v00001-第一卷-废材逆袭/ch00001-第1章-陨落.md`
	args := `{"file_path":"` + path + `","content":"第一行\n第二行"}`

	got := mustForwardSSEEvent(t, collector, handler, agent.Event{Type: "tool_call", Data: map[string]interface{}{
		"agent_kind": agent.AgentKindIDE,
		"id":         "call-1",
		"name":       "write_file",
		"args":       args,
	}})

	gotArgs := eventDataString(got.Data, "args")
	want := `{"file_path":"` + path + `"}`
	if gotArgs != want {
		t.Fatalf("display args should keep only absolute Nova chapter path, got %q", gotArgs)
	}
	if strings.Contains(gotArgs, "第一行") || strings.Contains(gotArgs, "content") || strings.Contains(gotArgs, "...") {
		t.Fatalf("display args should not include body or placeholder, got %q", gotArgs)
	}
	assertChapterBodyHiddenNotice(t, got.Data)
}

func TestSSEWriteFileChapterBodyMiddlewareShowsOnlyPathForPastedDetailArgs(t *testing.T) {
	collector, handler := newWriteFileChapterBodySSETestHandler()
	path := `/Users/huangyongquan/.codex/worktrees/999d/nova/.nova/测试/chapters/v00001-第一卷-废材逆袭/ch00011-第11章-水乳交融.md`
	args := `"file_path": "` + path + `", "content": "第一行\n第二行"`

	got := mustForwardSSEEvent(t, collector, handler, agent.Event{Type: "tool_call", Data: map[string]interface{}{
		"agent_kind": agent.AgentKindIDE,
		"id":         "call-1",
		"name":       "write_file",
		"args":       args,
	}})

	gotArgs := eventDataString(got.Data, "args")
	want := `{"file_path":"` + path + `"}`
	if gotArgs != want {
		t.Fatalf("display args should keep only pasted absolute Nova chapter path, got %q", gotArgs)
	}
	if strings.Contains(gotArgs, "第一行") || strings.Contains(gotArgs, "content") || strings.Contains(gotArgs, "...") {
		t.Fatalf("display args should not include body or placeholder, got %q", gotArgs)
	}
	assertChapterBodyHiddenNotice(t, got.Data)
}

func TestSSEWriteFileChapterBodyMiddlewareUsesTargetWhenArgsCannotRevealPath(t *testing.T) {
	collector, handler := newWriteFileChapterBodySSETestHandler()
	path := `/Users/huangyongquan/.codex/worktrees/999d/nova/.nova/测试/chapters/v00001/ch00001.md`
	args := `{"content":"第一行\n第二行`

	got := mustForwardSSEEvent(t, collector, handler, agent.Event{Type: "tool_call", Data: map[string]interface{}{
		"agent_kind": agent.AgentKindIDE,
		"id":         "call-1",
		"name":       "write_file",
		"args":       args,
		"target":     path,
	}})

	gotArgs := eventDataString(got.Data, "args")
	want := `{"file_path":"` + path + `"}`
	if gotArgs != want {
		t.Fatalf("display args should use target path when args cannot reveal path, got %q", gotArgs)
	}
	if strings.Contains(gotArgs, "第一行") || strings.Contains(gotArgs, "content") || strings.Contains(gotArgs, "...") {
		t.Fatalf("display args should not include body or placeholder, got %q", gotArgs)
	}
	assertChapterBodyHiddenNotice(t, got.Data)
}

func TestSSEWriteFileChapterBodyMiddlewareHoldsUnknownToolCallArgs(t *testing.T) {
	collector, handler := newWriteFileChapterBodySSETestHandler()

	got := mustForwardSSEEvent(t, collector, handler, agent.Event{Type: "tool_call", Data: map[string]interface{}{
		"agent_kind": agent.AgentKindIDE,
		"id":         "call-1",
		"name":       "write_file",
		"args":       `{"content":"第一行`,
	}})

	if gotArgs := eventDataString(got.Data, "args"); gotArgs != "" {
		t.Fatalf("unknown write_file args should be held from SSE, got %q", gotArgs)
	}
}

func TestSSEWriteFileChapterBodyMiddlewareProjectsToolTargetToArgsDelta(t *testing.T) {
	collector, handler := newWriteFileChapterBodySSETestHandler()
	path := `/Users/huangyongquan/.codex/worktrees/999d/nova/.nova/测试/chapters/v00001/ch00001.md`
	_ = mustForwardSSEEvent(t, collector, handler, agent.Event{Type: "tool_call", Data: map[string]interface{}{
		"agent_kind": agent.AgentKindIDE,
		"id":         "call-1",
		"name":       "write_file",
		"args":       "",
	}})

	got := mustForwardSSEEvent(t, collector, handler, agent.Event{Type: "tool_target", Data: map[string]interface{}{
		"agent_kind": agent.AgentKindIDE,
		"id":         "call-1",
		"name":       "write_file",
		"target":     path,
	}})
	mustSuppressSSEEvent(t, collector, handler, agent.Event{Type: "tool_args_delta", Data: map[string]interface{}{
		"agent_kind": agent.AgentKindIDE,
		"id":         "call-1",
		"name":       "write_file",
		"delta":      `{"content":"第一行`,
	}})

	if got.Type != "tool_args_delta" {
		t.Fatalf("tool_target should be projected to tool_args_delta, got %q", got.Type)
	}
	gotDelta := eventDataString(got.Data, "delta")
	want := `{"file_path":"` + path + `"}`
	if gotDelta != want {
		t.Fatalf("projected target delta = %q, want %q", gotDelta, want)
	}
	assertChapterBodyHiddenNotice(t, got.Data)
}

func TestSSEWriteFileChapterBodyMiddlewareDropsChapterContentDeltas(t *testing.T) {
	collector, handler := newWriteFileChapterBodySSETestHandler()
	_ = mustForwardSSEEvent(t, collector, handler, agent.Event{Type: "tool_call", Data: map[string]interface{}{
		"agent_kind": agent.AgentKindIDE,
		"id":         "call-1",
		"name":       "write_file",
		"args":       "",
	}})

	first := mustForwardSSEEvent(t, collector, handler, agent.Event{Type: "tool_args_delta", Data: map[string]interface{}{
		"agent_kind": agent.AgentKindIDE,
		"id":         "call-1",
		"name":       "write_file",
		"delta":      `{"file_path":"chapters/ch02.md","content":"第一行`,
	}})
	mustSuppressSSEEvent(t, collector, handler, agent.Event{Type: "tool_args_delta", Data: map[string]interface{}{
		"agent_kind": agent.AgentKindIDE,
		"id":         "call-1",
		"name":       "write_file",
		"delta":      `\n第二行\n第三行"}`,
	}})

	firstDelta := eventDataString(first.Data, "delta")
	if firstDelta != `{"file_path":"chapters/ch02.md"}` {
		t.Fatalf("first display delta should include only path: %q", firstDelta)
	}
	if strings.Contains(firstDelta, "第一行") || strings.Contains(firstDelta, "content") || strings.Contains(firstDelta, "...") {
		t.Fatalf("first display delta should not include body or placeholder: %q", firstDelta)
	}
	assertChapterBodyHiddenNotice(t, first.Data)
}

func TestSSEWriteFileChapterBodyMiddlewareDropsDraftContentDeltas(t *testing.T) {
	collector, handler := newWriteFileChapterBodySSETestHandler()
	_ = mustForwardSSEEvent(t, collector, handler, agent.Event{Type: "tool_call", Data: map[string]interface{}{
		"agent_kind": agent.AgentKindIDE,
		"id":         "call-1",
		"name":       "write_file",
		"args":       "",
	}})

	first := mustForwardSSEEvent(t, collector, handler, agent.Event{Type: "tool_args_delta", Data: map[string]interface{}{
		"agent_kind": agent.AgentKindIDE,
		"id":         "call-1",
		"name":       "write_file",
		"delta":      `{"file_path":"drafts/ch02.md","content":"第一行`,
	}})

	if got := eventDataString(first.Data, "delta"); got != `{"file_path":"drafts/ch02.md"}` {
		t.Fatalf("draft display delta should include only path: %q", got)
	}
	assertChapterBodyHiddenNotice(t, first.Data)
}

func TestSSEWriteFileChapterBodyMiddlewareDropsAbsoluteNovaChapterContentDeltas(t *testing.T) {
	collector, handler := newWriteFileChapterBodySSETestHandler()
	_ = mustForwardSSEEvent(t, collector, handler, agent.Event{Type: "tool_call", Data: map[string]interface{}{
		"agent_kind": agent.AgentKindIDE,
		"id":         "call-1",
		"name":       "write_file",
		"args":       "",
	}})

	first := mustForwardSSEEvent(t, collector, handler, agent.Event{Type: "tool_args_delta", Data: map[string]interface{}{
		"agent_kind": agent.AgentKindIDE,
		"id":         "call-1",
		"name":       "write_file",
		"delta":      `{"file_path":"/Users/huangyongquan/.codex/worktrees/999d/nova/.nova/测试/chapters/v00001-第一卷-废材逆袭/ch00001-第1章-陨落.md","content":"第一行`,
	}})
	mustSuppressSSEEvent(t, collector, handler, agent.Event{Type: "tool_args_delta", Data: map[string]interface{}{
		"agent_kind": agent.AgentKindIDE,
		"id":         "call-1",
		"name":       "write_file",
		"delta":      `\n第二行"}`,
	}})

	firstDelta := eventDataString(first.Data, "delta")
	if !strings.Contains(firstDelta, `.nova/测试/chapters/`) {
		t.Fatalf("absolute Nova chapter delta should include path: %q", firstDelta)
	}
	if strings.Contains(firstDelta, "第一行") || strings.Contains(firstDelta, "content") || strings.Contains(firstDelta, "...") {
		t.Fatalf("absolute Nova chapter delta should not include body or placeholder: %q", firstDelta)
	}
	assertChapterBodyHiddenNotice(t, first.Data)
}

func TestSSEWriteFileChapterBodyMiddlewareRestoresNonChapterDeltas(t *testing.T) {
	collector, handler := newWriteFileChapterBodySSETestHandler()
	_ = mustForwardSSEEvent(t, collector, handler, agent.Event{Type: "tool_call", Data: map[string]interface{}{
		"agent_kind": agent.AgentKindIDE,
		"id":         "call-1",
		"name":       "write_file",
		"args":       "",
	}})

	mustSuppressSSEEvent(t, collector, handler, agent.Event{Type: "tool_args_delta", Data: map[string]interface{}{
		"agent_kind": agent.AgentKindIDE,
		"id":         "call-1",
		"name":       "write_file",
		"delta":      `{"file_path":"set`,
	}})
	next := mustForwardSSEEvent(t, collector, handler, agent.Event{Type: "tool_args_delta", Data: map[string]interface{}{
		"agent_kind": agent.AgentKindIDE,
		"id":         "call-1",
		"name":       "write_file",
		"delta":      `ting/outline.md","content":"第一行`,
	}})

	got := eventDataString(next.Data, "delta")
	if !strings.Contains(got, `{"file_path":"setting/outline.md"`) || !strings.Contains(got, "第一行") {
		t.Fatalf("non-chapter delta should restore held args, got %q", got)
	}
}

func TestSSEWriteFileChapterBodyMiddlewareKeepsConfigManagerWriteFileDeltas(t *testing.T) {
	collector, handler := newWriteFileChapterBodySSETestHandler()
	_ = mustForwardSSEEvent(t, collector, handler, agent.Event{Type: "tool_call", Data: map[string]interface{}{
		"agent_kind": agent.AgentKindConfigManager,
		"id":         "call-1",
		"name":       "write_file",
		"args":       "",
	}})

	next := mustForwardSSEEvent(t, collector, handler, agent.Event{Type: "tool_args_delta", Data: map[string]interface{}{
		"agent_kind": agent.AgentKindConfigManager,
		"id":         "call-1",
		"name":       "write_file",
		"delta":      `{"file_path":"chapters/ch02.md","content":"第一行`,
	}})

	if got := eventDataString(next.Data, "delta"); !strings.Contains(got, "第一行") {
		t.Fatalf("config_manager delta should stay unchanged: %q", got)
	}
}

func TestSSEWriteFileChapterBodyMiddlewareKeepsEditFileChapterDeltas(t *testing.T) {
	collector, handler := newWriteFileChapterBodySSETestHandler()
	_ = mustForwardSSEEvent(t, collector, handler, agent.Event{Type: "tool_call", Data: map[string]interface{}{
		"agent_kind": agent.AgentKindIDE,
		"id":         "call-1",
		"name":       "edit_file",
		"args":       "",
	}})

	next := mustForwardSSEEvent(t, collector, handler, agent.Event{Type: "tool_args_delta", Data: map[string]interface{}{
		"agent_kind": agent.AgentKindIDE,
		"id":         "call-1",
		"name":       "edit_file",
		"delta":      `{"file_path":"chapters/ch02.md","new_string":"第一行`,
	}})

	if got := eventDataString(next.Data, "delta"); !strings.Contains(got, "第一行") {
		t.Fatalf("edit_file delta should stay unchanged: %q", got)
	}
}

func TestIsNovelChapterBodyPath(t *testing.T) {
	cases := []struct {
		name string
		path string
		want bool
	}{
		{name: "relative chapter", path: "chapters/ch01.md", want: true},
		{name: "relative draft", path: "./drafts/ch01.md", want: true},
		{name: "absolute nova chapter", path: "/Users/me/nova/.nova/测试/chapters/ch01.md", want: true},
		{name: "absolute nova draft", path: `/Users\me\nova\.nova\测试\drafts\ch01.md`, want: true},
		{name: "absolute unrelated chapter directory", path: "/Users/me/tmp/chapters/ch01.md", want: false},
		{name: "nova setting", path: "/Users/me/nova/.nova/测试/setting/outline.md", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isNovelChapterBodyPath(tc.path); got != tc.want {
				t.Fatalf("isNovelChapterBodyPath(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func newWriteFileChapterBodySSETestHandler() (*sseEventCollector, SSEEventHandler) {
	collector := &sseEventCollector{}
	chain := newSSEEventMiddlewareChainWithMiddlewares(newWriteFileChapterBodySSEMiddleware())
	return collector, chain.Next(collector.Handle)
}

func assertChapterBodyHiddenNotice(t *testing.T, data interface{}) {
	t.Helper()
	fields, ok := data.(map[string]interface{})["sse_hidden_fields"].([]string)
	if !ok || len(fields) != 1 || fields[0] != "content" {
		t.Fatalf("sse_hidden_fields = %#v, want [content]", data.(map[string]interface{})["sse_hidden_fields"])
	}
	if got := eventDataString(data, "sse_hidden_reason"); got != chapterBodyHiddenReason {
		t.Fatalf("sse_hidden_reason = %q, want %q", got, chapterBodyHiddenReason)
	}
	if got := eventDataString(data, "sse_display_notice"); got != chapterBodyHiddenNotice {
		t.Fatalf("sse_display_notice = %q, want %q", got, chapterBodyHiddenNotice)
	}
}
