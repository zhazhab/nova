package middleware

import (
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
	rawArgs        string
	displayArgs    string
	isChapterBody  bool
	generatedChars int
	sentChars      int
	contentCounter jsonStringFieldCounter
}

func newWriteFileChapterBodySSEMiddleware() *writeFileChapterBodySSEMiddleware {
	return &writeFileChapterBodySSEMiddleware{
		toolArgs: map[string]*writeFileChapterBodySSEState{},
	}
}

func newWriteFileChapterBodySSEState(args string) *writeFileChapterBodySSEState {
	state := &writeFileChapterBodySSEState{
		contentCounter: newJSONStringFieldCounter("content"),
	}
	state.appendArgs(args)
	return state
}

func (s *writeFileChapterBodySSEState) appendArgs(delta string) {
	if delta == "" {
		return
	}
	s.rawArgs += delta
	s.generatedChars += s.contentCounter.Write(delta)
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
	state := newWriteFileChapterBodySSEState(args)
	if id != "" {
		m.toolArgs[id] = state
	}
	if target != "" {
		displayArgs, ok := m.projectNovelPath(target)
		if !ok {
			return next(ev)
		}
		state.isChapterBody = true
		state.displayArgs = displayArgs
		state.sentChars = state.generatedChars
		data := cloneEventDataMap(ev.Data)
		data["args"] = displayArgs
		return next(agent.Event{Type: ev.Type, Data: markChapterBodyHidden(data, state.generatedChars)})
	}
	if args == "" {
		return next(ev)
	}
	displayArgs, isChapterBody, ok := m.projectArgs(args)
	if !ok {
		data := cloneEventDataMap(ev.Data)
		data["args"] = ""
		return next(agent.Event{Type: ev.Type, Data: data})
	}
	if displayArgs == args {
		state.displayArgs = displayArgs
		return next(ev)
	}
	state.isChapterBody = isChapterBody
	state.displayArgs = displayArgs
	state.sentChars = state.generatedChars
	data := cloneEventDataMap(ev.Data)
	data["args"] = displayArgs
	return next(agent.Event{Type: ev.Type, Data: markChapterBodyHidden(data, state.generatedChars)})
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
		state = newWriteFileChapterBodySSEState("")
		m.toolArgs[id] = state
	}
	state.isChapterBody = true
	displayDelta := toolArgsDisplayDelta(state.displayArgs, displayArgs)
	state.displayArgs = displayArgs
	charsChanged := state.generatedChars != state.sentChars
	if displayDelta == "" && !charsChanged {
		return nil
	}
	state.sentChars = state.generatedChars
	data := cloneEventDataMap(ev.Data)
	delete(data, "target")
	data["delta"] = displayDelta
	return next(agent.Event{Type: "tool_args_delta", Data: markChapterBodyHidden(data, state.generatedChars)})
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
		state = newWriteFileChapterBodySSEState("")
		m.toolArgs[id] = state
	}
	state.appendArgs(delta)
	if state.isChapterBody {
		return m.forwardChapterBodyProgressDelta(ev, next, state)
	}
	displayArgs, isChapterBody, ok := m.projectArgs(state.rawArgs)
	if !ok {
		return nil
	}
	state.isChapterBody = isChapterBody
	if state.isChapterBody {
		return m.forwardChapterBodyProgressDelta(ev, next, state)
	}
	displayDelta := toolArgsDisplayDelta(state.displayArgs, displayArgs)
	state.displayArgs = displayArgs
	if displayDelta == "" {
		return nil
	}
	data := cloneEventDataMap(ev.Data)
	data["delta"] = displayDelta
	return next(agent.Event{Type: ev.Type, Data: data})
}

func (m *writeFileChapterBodySSEMiddleware) forwardChapterBodyProgressDelta(ev agent.Event, next SSEEventHandler, state *writeFileChapterBodySSEState) error {
	displayArgs := state.displayArgs
	if displayArgs == "" {
		projectedArgs, _, ok := m.projectArgs(state.rawArgs)
		if ok {
			displayArgs = projectedArgs
		}
	}
	displayDelta := toolArgsDisplayDelta(state.displayArgs, displayArgs)
	state.displayArgs = displayArgs
	charsChanged := state.generatedChars != state.sentChars
	if displayDelta == "" && !charsChanged {
		return nil
	}
	state.sentChars = state.generatedChars
	data := cloneEventDataMap(ev.Data)
	data["delta"] = displayDelta
	return next(agent.Event{Type: ev.Type, Data: markChapterBodyHidden(data, state.generatedChars)})
}

func (m *writeFileChapterBodySSEMiddleware) shouldProcessTool(ev agent.Event, name string) bool {
	return eventDataString(ev.Data, "agent_kind") == agent.AgentKindIDE && name == "write_file"
}

func (m *writeFileChapterBodySSEMiddleware) projectArgs(args string) (string, bool, bool) {
	preview, ok := toolPathArgPreviewFromArgs(args)
	if !ok {
		return "", false, false
	}
	if !isNovelChapterBodyPath(preview.path) {
		return args, false, true
	}
	return marshalToolPathArgPreview(preview), true, true
}

func (m *writeFileChapterBodySSEMiddleware) projectNovelPath(path string) (string, bool) {
	path = strings.TrimSpace(path)
	if path == "" || !isNovelChapterBodyPath(path) {
		return "", false
	}
	return marshalToolPathArgPreview(toolPathArgPreview{key: "file_path", path: path}), true
}

func markChapterBodyHidden(data map[string]interface{}, generatedChars int) map[string]interface{} {
	data["sse_hidden_fields"] = []string{"content"}
	data["sse_hidden_reason"] = chapterBodyHiddenReason
	data["sse_display_notice"] = chapterBodyHiddenNotice
	data["sse_generated_chars"] = generatedChars
	return data
}
