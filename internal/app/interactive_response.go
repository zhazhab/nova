package app

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"nova/internal/interactive"
)

const (
	narrativeStartTag  = "<NARRATIVE>"
	narrativeEndTag    = "</NARRATIVE>"
	hotStateStartTag   = "<HOT_STATE>"
	hotStateEndTag     = "</HOT_STATE>"
	stateDeltaStartTag = "<STATE_DELTA>"
	stateDeltaEndTag   = "</STATE_DELTA>"
)

type interactiveStatePayload struct {
	Ops []interactive.StateOp `json:"ops"`
}

type interactiveHotStatePayload struct {
	Choices []string `json:"choices"`
}

func parseInteractiveAssistantOutput(content string) (string, []interactive.StateOp, *interactive.HotState, error) {
	narrative := extractNarrative(content)
	if strings.TrimSpace(narrative) == "" {
		return "", nil, nil, fmt.Errorf("互动叙事内容为空")
	}

	stateBlock, ok := extractBetween(content, stateDeltaStartTag, stateDeltaEndTag)
	hotState, hotErr := parseInteractiveHotState(content)
	if hotErr != nil {
		log.Printf("[interactive-agent] parse hot state failed err=%v content=%q", hotErr, content)
	}
	if !ok || strings.TrimSpace(stateBlock) == "" {
		return strings.TrimSpace(narrative), nil, hotState, nil
	}
	ops, err := parseInteractiveStateOps(stateBlock)
	if err != nil {
		return strings.TrimSpace(narrative), nil, hotState, nil
	}
	return strings.TrimSpace(narrative), ops, hotState, nil
}

func parseInteractiveStateOps(content string) ([]interactive.StateOp, error) {
	var payload interactiveStatePayload
	if err := json.Unmarshal([]byte(extractJSONPayload(content)), &payload); err != nil {
		return nil, fmt.Errorf("解析互动状态失败: %w", err)
	}
	if err := validateStateOps(payload.Ops); err != nil {
		return nil, err
	}
	return payload.Ops, nil
}

func parseInteractiveHotState(content string) (*interactive.HotState, error) {
	stateBlock, ok := extractBetween(content, hotStateStartTag, hotStateEndTag)
	if !ok || strings.TrimSpace(stateBlock) == "" {
		return nil, nil
	}
	var payload interactiveHotStatePayload
	if err := json.Unmarshal([]byte(extractJSONPayload(stateBlock)), &payload); err != nil {
		return nil, fmt.Errorf("解析互动热状态失败: %w", err)
	}
	choices := make([]string, 0, len(payload.Choices))
	seen := map[string]bool{}
	for _, choice := range payload.Choices {
		choice = strings.TrimSpace(choice)
		if choice == "" || seen[choice] {
			continue
		}
		choices = append(choices, choice)
		seen[choice] = true
		if len(choices) >= 5 {
			break
		}
	}
	if len(choices) == 0 {
		return nil, nil
	}
	return &interactive.HotState{Choices: choices}, nil
}

func extractNarrative(content string) string {
	if narrative, ok := extractBetween(content, narrativeStartTag, narrativeEndTag); ok {
		return narrative
	}
	if idx := firstHiddenStateTagIndex(content); idx >= 0 {
		return content[:idx]
	}
	return content
}

func firstHiddenStateTagIndex(content string) int {
	indexes := []int{
		strings.Index(content, hotStateStartTag),
		strings.Index(content, stateDeltaStartTag),
	}
	best := -1
	for _, idx := range indexes {
		if idx >= 0 && (best < 0 || idx < best) {
			best = idx
		}
	}
	return best
}

func extractBetween(content, startTag, endTag string) (string, bool) {
	start := strings.Index(content, startTag)
	if start < 0 {
		return "", false
	}
	start += len(startTag)
	end := strings.Index(content[start:], endTag)
	if end < 0 {
		return content[start:], true
	}
	return content[start : start+end], true
}

func extractJSONPayload(content string) string {
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSpace(content)
		content = strings.TrimSuffix(content, "```")
	}
	return strings.TrimSpace(content)
}

func validateStateOps(ops []interactive.StateOp) error {
	if len(ops) == 0 {
		return fmt.Errorf("互动状态变化不能为空：STATE_DELTA.ops 至少需要一条本回合状态变化")
	}
	for _, op := range ops {
		switch op.Op {
		case "set", "merge", "push", "pull", "inc", "unset":
		default:
			return fmt.Errorf("不支持的互动状态操作: %s", op.Op)
		}
		if !isAllowedStatePath(op.Path) {
			return fmt.Errorf("不支持的互动状态路径: %s", op.Path)
		}
	}
	return nil
}

func isAllowedStatePath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	allowedRoots := []string{
		"on_stage",
		"characters",
		"events",
		"location",
		"time",
		"pov",
		"scene",
		"inventory",
		"resources",
		"world_flags",
		"rules",
		"threads",
		"action_space",
	}
	for _, root := range allowedRoots {
		if path == root || strings.HasPrefix(path, root+".") {
			return true
		}
	}
	return false
}
