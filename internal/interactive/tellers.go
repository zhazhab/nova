package interactive

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const tellerVersion = 1

type TellerLibrary struct {
	novaDir string
}

type Teller struct {
	Version         int                 `json:"version"`
	ID              string              `json:"id"`
	Name            string              `json:"name"`
	Description     string              `json:"description"`
	RandomEventRate float64             `json:"random_event_rate"`
	Tags            []string            `json:"tags"`
	ContextPolicy   TellerContextPolicy `json:"context_policy"`
	Slots           []TellerPromptSlot  `json:"slots"`
	Path            string              `json:"path,omitempty"`
	Custom          bool                `json:"custom"`
	Invalid         bool                `json:"invalid,omitempty"`
	Error           string              `json:"error,omitempty"`
	CreatedAt       string              `json:"created_at,omitempty"`
	UpdatedAt       string              `json:"updated_at,omitempty"`
}

type TellerContextPolicy struct {
	Creator      string `json:"creator"`
	Lore         string `json:"lore"`
	RuntimeState string `json:"runtime_state"`
	RecentTurns  int    `json:"recent_turns"`
}

type TellerPromptSlot struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Target  string `json:"target"`
	Enabled bool   `json:"enabled"`
	Content string `json:"content"`
}

func NewTellerLibrary(novaDir string) *TellerLibrary {
	return &TellerLibrary{novaDir: novaDir}
}

func (l *TellerLibrary) List() ([]Teller, error) {
	if err := l.ensureBuiltins(); err != nil {
		return nil, err
	}
	files, err := filepath.Glob(filepath.Join(l.dir(), "*.json"))
	if err != nil {
		return nil, err
	}
	tellers := make([]Teller, 0, len(files))
	for _, file := range files {
		teller, err := parseTellerFile(file)
		if err != nil {
			tellers = append(tellers, Teller{
				ID:      strings.TrimSuffix(filepath.Base(file), ".json"),
				Path:    file,
				Invalid: true,
				Error:   err.Error(),
				Custom:  !isBuiltinTellerFile(file),
			})
			continue
		}
		teller.Path = file
		teller.Custom = !isBuiltinID(teller.ID)
		tellers = append(tellers, teller)
	}
	sort.Slice(tellers, func(i, j int) bool {
		if tellers[i].Custom != tellers[j].Custom {
			return !tellers[i].Custom
		}
		return tellers[i].ID < tellers[j].ID
	})
	return tellers, nil
}

func (l *TellerLibrary) Get(id string) (Teller, error) {
	if err := l.ensureBuiltins(); err != nil {
		return Teller{}, err
	}
	if err := validateTellerID(id); err != nil {
		return Teller{}, err
	}
	teller, err := parseTellerFile(filepath.Join(l.dir(), id+".json"))
	if err != nil {
		return Teller{}, err
	}
	teller.Custom = !isBuiltinID(teller.ID)
	return teller, nil
}

func (l *TellerLibrary) Create(teller Teller) (Teller, error) {
	if err := l.ensureBuiltins(); err != nil {
		return Teller{}, err
	}
	teller = normalizeTeller(teller)
	if teller.ID == "" {
		teller.ID = newTellerID()
	}
	if err := validateTeller(teller); err != nil {
		return Teller{}, err
	}
	path := filepath.Join(l.dir(), teller.ID+".json")
	if _, err := os.Stat(path); err == nil {
		return Teller{}, fmt.Errorf("讲述者 ID 已存在: %s", teller.ID)
	} else if !os.IsNotExist(err) {
		return Teller{}, err
	}
	now := time.Now().Format(time.RFC3339)
	teller.CreatedAt = now
	teller.UpdatedAt = now
	if err := writeTellerFile(path, teller); err != nil {
		return Teller{}, err
	}
	teller.Path = path
	teller.Custom = !isBuiltinID(teller.ID)
	return teller, nil
}

func (l *TellerLibrary) Update(id string, teller Teller) (Teller, error) {
	if err := l.ensureBuiltins(); err != nil {
		return Teller{}, err
	}
	if err := validateTellerID(id); err != nil {
		return Teller{}, err
	}
	current, err := l.Get(id)
	if err != nil {
		return Teller{}, err
	}
	teller.ID = id
	teller.CreatedAt = current.CreatedAt
	teller.UpdatedAt = time.Now().Format(time.RFC3339)
	teller = normalizeTeller(teller)
	if err := validateTeller(teller); err != nil {
		return Teller{}, err
	}
	path := filepath.Join(l.dir(), id+".json")
	if err := writeTellerFile(path, teller); err != nil {
		return Teller{}, err
	}
	teller.Path = path
	teller.Custom = !isBuiltinID(teller.ID)
	return teller, nil
}

func (l *TellerLibrary) Delete(id string) error {
	if err := validateTellerID(id); err != nil {
		return err
	}
	if isBuiltinID(id) {
		return errors.New("内置讲述者不能删除")
	}
	return os.Remove(filepath.Join(l.dir(), id+".json"))
}

func (l *TellerLibrary) dir() string {
	return filepath.Join(l.novaDir, "story-tellers")
}

func (l *TellerLibrary) ensureBuiltins() error {
	if err := os.MkdirAll(l.dir(), 0o755); err != nil {
		return err
	}
	for id, teller := range builtinTellers {
		path := filepath.Join(l.dir(), id+".json")
		if _, err := os.Stat(path); err == nil {
			continue
		}
		if err := writeTellerFile(path, teller); err != nil {
			return err
		}
	}
	return nil
}

func parseTellerFile(path string) (Teller, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Teller{}, err
	}
	var teller Teller
	if err := json.Unmarshal(data, &teller); err != nil {
		return Teller{}, fmt.Errorf("解析讲述者 JSON 失败: %w", err)
	}
	teller = normalizeTeller(teller)
	if err := validateTeller(teller); err != nil {
		return Teller{}, err
	}
	teller.Path = path
	return teller, nil
}

func writeTellerFile(path string, teller Teller) error {
	teller = normalizeTeller(teller)
	data, err := json.MarshalIndent(teller, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func (t Teller) PromptForTargets(targets ...string) string {
	allowed := map[string]bool{}
	for _, target := range targets {
		allowed[target] = true
	}
	var sb strings.Builder
	for _, slot := range t.Slots {
		if !slot.Enabled || !allowed[slot.Target] || strings.TrimSpace(slot.Content) == "" {
			continue
		}
		fmt.Fprintf(&sb, "## %s（%s）\n\n%s\n\n", slot.Name, slot.Target, strings.TrimSpace(slot.Content))
	}
	return strings.TrimSpace(sb.String())
}

func normalizeTeller(teller Teller) Teller {
	teller.Version = tellerVersion
	teller.ID = strings.TrimSpace(teller.ID)
	teller.Name = strings.TrimSpace(teller.Name)
	teller.Description = strings.TrimSpace(teller.Description)
	teller.Tags = normalizeTellerTags(teller.Tags)
	teller.ContextPolicy = normalizeContextPolicy(teller.ContextPolicy)
	teller.Slots = normalizePromptSlots(teller.Slots)
	return teller
}

func normalizeContextPolicy(policy TellerContextPolicy) TellerContextPolicy {
	if strings.TrimSpace(policy.Creator) == "" {
		policy.Creator = "always"
	}
	if strings.TrimSpace(policy.Lore) == "" {
		policy.Lore = "relevant"
	}
	if strings.TrimSpace(policy.RuntimeState) == "" {
		policy.RuntimeState = "always"
	}
	if policy.RecentTurns <= 0 {
		policy.RecentTurns = 8
	}
	return policy
}

func normalizePromptSlots(slots []TellerPromptSlot) []TellerPromptSlot {
	result := make([]TellerPromptSlot, 0, len(slots))
	seen := map[string]bool{}
	for _, slot := range slots {
		slot.ID = normalizeSlotID(slot.ID)
		if slot.ID == "" {
			slot.ID = fmt.Sprintf("slot-%d", len(result)+1)
		}
		if seen[slot.ID] {
			continue
		}
		seen[slot.ID] = true
		slot.Name = strings.TrimSpace(slot.Name)
		if slot.Name == "" {
			slot.Name = slot.ID
		}
		slot.Target = normalizeSlotTarget(slot.Target)
		slot.Content = strings.TrimSpace(slot.Content)
		result = append(result, slot)
	}
	return result
}

func validateTeller(teller Teller) error {
	if err := validateTellerID(teller.ID); err != nil {
		return err
	}
	if teller.Name == "" {
		return errors.New("讲述者名称不能为空")
	}
	if len(teller.Slots) == 0 {
		return errors.New("讲述者至少需要一个 prompt slot")
	}
	return nil
}

func validateTellerID(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("讲述者 ID 不能为空")
	}
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return fmt.Errorf("讲述者 ID 包含非法字符: %s", id)
	}
	return nil
}

func normalizeTellerTags(tags []string) []string {
	result := make([]string, 0, len(tags))
	seen := map[string]bool{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		result = append(result, tag)
	}
	return result
}

func normalizeSlotID(id string) string {
	id = strings.TrimSpace(id)
	var sb strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func normalizeSlotTarget(target string) string {
	switch strings.TrimSpace(target) {
	case "system", "context", "thinking", "private_instruction", "turn", "state_agent", "editor_agent":
		return strings.TrimSpace(target)
	default:
		return "context"
	}
}

func newTellerID() string {
	return fmt.Sprintf("teller-%d", time.Now().UTC().UnixNano())
}

func isBuiltinTellerFile(path string) bool {
	return isBuiltinID(strings.TrimSuffix(filepath.Base(path), ".json"))
}

func isBuiltinID(id string) bool {
	_, ok := builtinTellers[id]
	return ok
}

var builtinTellers = map[string]Teller{
	"classic": builtinTeller("classic", "经典叙事者", "平衡叙事，节奏稳定，少量随机事件", 0.15, []string{"通用", "平衡"}, []TellerPromptSlot{
		{ID: "identity", Name: "讲述者身份", Target: "system", Enabled: true, Content: "你是一位经典叙事者，注重故事节奏、角色选择与清晰的场景反馈。"},
		{ID: "deliberation", Name: "内部裁定规则", Target: "thinking", Enabled: true, Content: "每轮内部检查用户行动、在场角色、世界规则、因果代价和开放选择点；不要输出分析过程。"},
		{ID: "turn_rules", Name: "回合输出规则", Target: "turn", Enabled: true, Content: "正文要聚焦、有推进，并在结尾停留于可继续行动的选择点或悬念点。"},
		{ID: "state_rules", Name: "状态记录规则", Target: "state_agent", Enabled: true, Content: "只记录本回合已经发生且确定成立的状态变化，不记录未来计划。"},
	}),
	"grimdark": builtinTeller("grimdark", "黑暗低魔", "压抑氛围，强调代价、危险与残酷选择", 0.25, []string{"黑暗", "低魔"}, []TellerPromptSlot{
		{ID: "identity", Name: "讲述者身份", Target: "system", Enabled: true, Content: "你是一位黑暗低魔叙事者，偏好艰难抉择、稀缺资源、危险旅程和不可逆后果。"},
		{ID: "deliberation", Name: "内部裁定规则", Target: "thinking", Enabled: true, Content: "每轮都要检查行动代价、资源消耗、风险升级和势力反应；不要输出分析过程。"},
		{ID: "turn_rules", Name: "回合输出规则", Target: "turn", Enabled: true, Content: "避免轻易圆满解决冲突。成功也应留下代价、阴影、误解、伤势、债务或新的危险。"},
		{ID: "state_rules", Name: "状态记录规则", Target: "state_agent", Enabled: true, Content: "优先记录伤势、资源损耗、危险等级、势力敌意、未解决危机和角色心理压力。"},
	}),
	"lighthearted": builtinTeller("lighthearted", "轻松日常", "轻快温暖，偏向日常互动和角色关系", 0.1, []string{"日常", "轻松"}, []TellerPromptSlot{
		{ID: "identity", Name: "讲述者身份", Target: "system", Enabled: true, Content: "你是一位轻松日常叙事者，偏好温暖互动、幽默细节、人物关系变化和低压力事件。"},
		{ID: "deliberation", Name: "内部裁定规则", Target: "thinking", Enabled: true, Content: "每轮内部检查角色情绪、关系变化、生活细节和小小的行动回报；不要输出分析过程。"},
		{ID: "turn_rules", Name: "回合输出规则", Target: "turn", Enabled: true, Content: "正文可以更重对白、互动和细节反馈，让角色主动回应并留下自然的继续入口。"},
		{ID: "state_rules", Name: "状态记录规则", Target: "state_agent", Enabled: true, Content: "优先记录关系变化、情绪、承诺、共同经历、当前地点和可互动对象。"},
	}),
}

func builtinTeller(id, name, description string, randomEventRate float64, tags []string, slots []TellerPromptSlot) Teller {
	return normalizeTeller(Teller{
		Version:         tellerVersion,
		ID:              id,
		Name:            name,
		Description:     description,
		RandomEventRate: randomEventRate,
		Tags:            tags,
		ContextPolicy: TellerContextPolicy{
			Creator:      "always",
			Lore:         "relevant",
			RuntimeState: "always",
			RecentTurns:  8,
		},
		Slots: slots,
	})
}
