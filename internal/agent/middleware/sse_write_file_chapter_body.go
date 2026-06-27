package middleware

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"nova/internal/agent"
)

const (
	chapterBodyHiddenNotice = "chapter_body_hidden"
	chapterBodyHiddenReason = "novel_chapter_body"
)

type writeFileChapterBodySSEMiddleware struct {
	toolArgs map[string]*writeFileChapterBodySSEState
}

type writeFileChapterBodySSEState struct {
	rawArgs     string
	displayArgs string
}

func newWriteFileChapterBodySSEMiddleware() *writeFileChapterBodySSEMiddleware {
	return &writeFileChapterBodySSEMiddleware{
		toolArgs: map[string]*writeFileChapterBodySSEState{},
	}
}

func (m *writeFileChapterBodySSEMiddleware) Next(next SSEEventHandler) SSEEventHandler {
	if next == nil {
		next = func(agent.Event) error { return nil }
	}
	return func(ev agent.Event) error {
		if m == nil {
			return next(ev)
		}
		switch ev.Type {
		case "tool_call":
			return m.processToolCall(ev, next)
		case "tool_target":
			return m.processToolTarget(ev, next)
		case "tool_args_delta":
			return m.processToolArgsDelta(ev, next)
		case "tool_result", "error", "aborted":
			if id := eventDataString(ev.Data, "id"); id != "" {
				delete(m.toolArgs, id)
			}
			return next(ev)
		case "done":
			m.toolArgs = map[string]*writeFileChapterBodySSEState{}
			return next(ev)
		default:
			return next(ev)
		}
	}
}

func (m *writeFileChapterBodySSEMiddleware) processToolCall(ev agent.Event, next SSEEventHandler) error {
	id := eventDataString(ev.Data, "id")
	name := eventDataString(ev.Data, "name")
	args := eventDataString(ev.Data, "args")
	target := eventDataString(ev.Data, "target")
	if !m.shouldProcessTool(ev, name) {
		return next(ev)
	}
	state := &writeFileChapterBodySSEState{rawArgs: args}
	if id != "" {
		m.toolArgs[id] = state
	}
	if target != "" {
		displayArgs, ok := m.projectNovelPath(target)
		if !ok {
			return next(ev)
		}
		state.displayArgs = displayArgs
		data := cloneEventDataMap(ev.Data)
		data["args"] = displayArgs
		return next(agent.Event{Type: ev.Type, Data: markChapterBodyHidden(data)})
	}
	if args == "" {
		return next(ev)
	}
	displayArgs, ok := m.projectArgs(args)
	if !ok {
		data := cloneEventDataMap(ev.Data)
		data["args"] = ""
		return next(agent.Event{Type: ev.Type, Data: data})
	}
	if displayArgs == args {
		state.displayArgs = displayArgs
		return next(ev)
	}
	state.displayArgs = displayArgs
	data := cloneEventDataMap(ev.Data)
	data["args"] = displayArgs
	return next(agent.Event{Type: ev.Type, Data: markChapterBodyHidden(data)})
}

func (m *writeFileChapterBodySSEMiddleware) processToolTarget(ev agent.Event, next SSEEventHandler) error {
	id := eventDataString(ev.Data, "id")
	name := eventDataString(ev.Data, "name")
	target := eventDataString(ev.Data, "target")
	if id == "" || target == "" || !m.shouldProcessTool(ev, name) {
		return next(ev)
	}
	displayArgs, ok := m.projectNovelPath(target)
	if !ok {
		return next(ev)
	}
	state := m.toolArgs[id]
	if state == nil {
		state = &writeFileChapterBodySSEState{}
		m.toolArgs[id] = state
	}
	displayDelta := toolArgsDisplayDelta(state.displayArgs, displayArgs)
	state.displayArgs = displayArgs
	if displayDelta == "" {
		return nil
	}
	data := cloneEventDataMap(ev.Data)
	delete(data, "target")
	data["delta"] = displayDelta
	return next(agent.Event{Type: "tool_args_delta", Data: markChapterBodyHidden(data)})
}

func (m *writeFileChapterBodySSEMiddleware) processToolArgsDelta(ev agent.Event, next SSEEventHandler) error {
	id := eventDataString(ev.Data, "id")
	name := eventDataString(ev.Data, "name")
	delta := eventDataString(ev.Data, "delta")
	if id == "" || delta == "" || !m.shouldProcessTool(ev, name) {
		return next(ev)
	}
	state := m.toolArgs[id]
	if state == nil {
		state = &writeFileChapterBodySSEState{}
		m.toolArgs[id] = state
	}
	state.rawArgs += delta
	displayArgs, ok := m.projectArgs(state.rawArgs)
	if !ok {
		return nil
	}
	displayDelta := toolArgsDisplayDelta(state.displayArgs, displayArgs)
	state.displayArgs = displayArgs
	if displayDelta == "" {
		return nil
	}
	data := cloneEventDataMap(ev.Data)
	data["delta"] = displayDelta
	return next(agent.Event{Type: ev.Type, Data: markChapterBodyHidden(data)})
}

func (m *writeFileChapterBodySSEMiddleware) shouldProcessTool(ev agent.Event, name string) bool {
	return eventDataString(ev.Data, "agent_kind") == agent.AgentKindIDE && name == "write_file"
}

func (m *writeFileChapterBodySSEMiddleware) projectArgs(args string) (string, bool) {
	preview, ok := toolPathArgPreviewFromArgs(args)
	if !ok {
		return "", false
	}
	if !isNovelChapterBodyPath(preview.path) {
		return args, true
	}
	return marshalToolPathArgPreview(preview), true
}

func (m *writeFileChapterBodySSEMiddleware) projectNovelPath(path string) (string, bool) {
	path = strings.TrimSpace(path)
	if path == "" || !isNovelChapterBodyPath(path) {
		return "", false
	}
	return marshalToolPathArgPreview(toolPathArgPreview{key: "file_path", path: path}), true
}

func markChapterBodyHidden(data map[string]interface{}) map[string]interface{} {
	data["sse_hidden_fields"] = []string{"content"}
	data["sse_hidden_reason"] = chapterBodyHiddenReason
	data["sse_display_notice"] = chapterBodyHiddenNotice
	return data
}

type toolPathArgPreview struct {
	key  string
	path string
}

func toolPathArgPreviewFromArgs(args string) (toolPathArgPreview, bool) {
	trimmed := strings.TrimSpace(args)
	if trimmed == "" {
		return toolPathArgPreview{}, false
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &payload); err == nil {
		for _, key := range []string{"file_path", "path", "filename", "file"} {
			value, _ := payload[key].(string)
			value = strings.TrimSpace(value)
			if value != "" {
				return toolPathArgPreview{key: displayToolPathKey(key), path: value}, true
			}
		}
	}
	for _, key := range []string{"file_path", "path", "filename", "file"} {
		value, ok := partialJSONStringField(trimmed, key)
		value = strings.TrimSpace(value)
		if ok && value != "" {
			return toolPathArgPreview{key: displayToolPathKey(key), path: value}, true
		}
	}
	return toolPathArgPreview{}, false
}

func marshalToolPathArgPreview(preview toolPathArgPreview) string {
	key := preview.key
	if key == "" {
		key = "path"
	}
	keyData, err := json.Marshal(key)
	if err != nil {
		return ""
	}
	pathData, err := json.Marshal(preview.path)
	if err != nil {
		return ""
	}
	return "{" + string(keyData) + ":" + string(pathData) + "}"
}

func displayToolPathKey(key string) string {
	switch key {
	case "file_path", "path":
		return key
	default:
		return "path"
	}
}

func partialJSONStringField(args, key string) (string, bool) {
	needle := `"` + key + `"`
	searchFrom := 0
	for {
		index := strings.Index(args[searchFrom:], needle)
		if index < 0 {
			return "", false
		}
		index += searchFrom
		afterKey := strings.TrimLeft(args[index+len(needle):], " \n\r\t")
		if !strings.HasPrefix(afterKey, ":") {
			searchFrom = index + len(needle)
			continue
		}
		afterColon := strings.TrimLeft(afterKey[1:], " \n\r\t")
		if !strings.HasPrefix(afterColon, `"`) {
			searchFrom = index + len(needle)
			continue
		}
		value := afterColon[1:]
		escaped := false
		for i := 0; i < len(value); i++ {
			switch value[i] {
			case '\\':
				escaped = !escaped
			case '"':
				if escaped {
					escaped = false
					continue
				}
				decoded, err := strconv.Unquote(`"` + value[:i] + `"`)
				if err != nil {
					return value[:i], true
				}
				return decoded, true
			default:
				escaped = false
			}
		}
		return "", false
	}
}

func isNovelChapterBodyPath(path string) bool {
	normalized := strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
	for strings.HasPrefix(normalized, "./") {
		normalized = strings.TrimPrefix(normalized, "./")
	}
	if strings.HasPrefix(normalized, "chapters/") || strings.HasPrefix(normalized, "drafts/") {
		return true
	}
	parts := strings.Split(normalized, "/")
	for index, part := range parts {
		if part != ".nova" || index+2 >= len(parts) {
			continue
		}
		if parts[index+2] == "chapters" || parts[index+2] == "drafts" {
			return true
		}
	}
	return false
}

func toolArgsDisplayDelta(previous, current string) string {
	if current == previous {
		return ""
	}
	if strings.HasPrefix(current, previous) {
		return strings.TrimPrefix(current, previous)
	}
	return current
}

func eventDataString(data interface{}, key string) string {
	switch typed := data.(type) {
	case map[string]string:
		return typed[key]
	case map[string]interface{}:
		if value, ok := typed[key]; ok {
			return fmt.Sprint(value)
		}
	}
	return ""
}

func cloneEventDataMap(data interface{}) map[string]interface{} {
	next := map[string]interface{}{}
	if typed, ok := data.(map[string]interface{}); ok {
		for key, value := range typed {
			next[key] = value
		}
		return next
	}
	if typed, ok := data.(map[string]string); ok {
		for key, value := range typed {
			next[key] = value
		}
	}
	return next
}
