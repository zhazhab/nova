package book

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

const loreItemsVersion = 1

// LoreItem 是用户可编辑的作品资料条目。固定字段只负责索引和展示，正文继续使用 Markdown。
type LoreItem struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Name       string   `json:"name"`
	Importance string   `json:"importance"`
	Tags       []string `json:"tags"`
	Content    string   `json:"content"`
	CreatedAt  string   `json:"created_at"`
	UpdatedAt  string   `json:"updated_at"`
}

type LoreItemInput struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Name       string   `json:"name"`
	Importance string   `json:"importance"`
	Tags       []string `json:"tags"`
	Content    string   `json:"content"`
}

type LoreCollection struct {
	Version        int        `json:"version"`
	LegacyMigrated bool       `json:"legacy_migrated"`
	Items          []LoreItem `json:"items"`
}

type LoreStore struct {
	workspace string
}

func NewLoreStore(workspace string) *LoreStore {
	return &LoreStore{workspace: workspace}
}

func (s *LoreStore) List() ([]LoreItem, error) {
	collection, err := s.loadOrCreate()
	if err != nil {
		return nil, err
	}
	items := append([]LoreItem(nil), collection.Items...)
	sort.SliceStable(items, func(i, j int) bool {
		if loreImportanceRank(items[i].Importance) != loreImportanceRank(items[j].Importance) {
			return loreImportanceRank(items[i].Importance) < loreImportanceRank(items[j].Importance)
		}
		if items[i].Type != items[j].Type {
			return items[i].Type < items[j].Type
		}
		return items[i].Name < items[j].Name
	})
	return items, nil
}

func (s *LoreStore) Create(input LoreItemInput) (LoreItem, error) {
	collection, err := s.loadOrCreate()
	if err != nil {
		return LoreItem{}, err
	}
	now := time.Now().Format(time.RFC3339)
	item := normalizeLoreItem(LoreItem{
		ID:         input.ID,
		Type:       input.Type,
		Name:       input.Name,
		Importance: input.Importance,
		Tags:       input.Tags,
		Content:    input.Content,
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	if item.ID == "" {
		item.ID = newLoreID(item.Type)
	}
	if item.Name == "" {
		return LoreItem{}, errors.New("资料名称不能为空")
	}
	if s.hasItem(collection.Items, item.ID) {
		return LoreItem{}, fmt.Errorf("资料 ID 已存在: %s", item.ID)
	}
	collection.Items = append(collection.Items, item)
	if err := s.save(collection); err != nil {
		return LoreItem{}, err
	}
	return item, nil
}

func (s *LoreStore) Update(id string, input LoreItemInput) (LoreItem, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return LoreItem{}, errors.New("资料 ID 不能为空")
	}
	collection, err := s.loadOrCreate()
	if err != nil {
		return LoreItem{}, err
	}
	for i := range collection.Items {
		if collection.Items[i].ID != id {
			continue
		}
		updated := normalizeLoreItem(LoreItem{
			ID:         id,
			Type:       input.Type,
			Name:       input.Name,
			Importance: input.Importance,
			Tags:       input.Tags,
			Content:    input.Content,
			CreatedAt:  collection.Items[i].CreatedAt,
			UpdatedAt:  time.Now().Format(time.RFC3339),
		})
		if updated.Name == "" {
			return LoreItem{}, errors.New("资料名称不能为空")
		}
		collection.Items[i] = updated
		if err := s.save(collection); err != nil {
			return LoreItem{}, err
		}
		return updated, nil
	}
	return LoreItem{}, fmt.Errorf("资料不存在: %s", id)
}

func (s *LoreStore) Delete(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("资料 ID 不能为空")
	}
	collection, err := s.loadOrCreate()
	if err != nil {
		return err
	}
	next := make([]LoreItem, 0, len(collection.Items))
	found := false
	for _, item := range collection.Items {
		if item.ID == id {
			found = true
			continue
		}
		next = append(next, item)
	}
	if !found {
		return fmt.Errorf("资料不存在: %s", id)
	}
	collection.Items = next
	return s.save(collection)
}

func (s *LoreStore) ContextMarkdown() (string, error) {
	items, err := s.List()
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, item := range items {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		fmt.Fprintf(&sb, "## %s（%s / %s）\n\n", item.Name, loreTypeLabel(item.Type), loreImportanceLabel(item.Importance))
		if len(item.Tags) > 0 {
			sb.WriteString("标签：")
			sb.WriteString(strings.Join(item.Tags, "、"))
			sb.WriteString("\n\n")
		}
		sb.WriteString(content)
		sb.WriteString("\n\n")
	}
	return strings.TrimSpace(sb.String()), nil
}

func (s *LoreStore) Ensure() error {
	_, err := s.loadOrCreate()
	return err
}

func (s *LoreStore) loadOrCreate() (LoreCollection, error) {
	path := s.itemsPath()
	data, err := os.ReadFile(path)
	if err == nil {
		var collection LoreCollection
		if err := json.Unmarshal(data, &collection); err != nil {
			return LoreCollection{}, fmt.Errorf("解析 lore items 失败: %w", err)
		}
		collection.Version = loreItemsVersion
		collection.Items = normalizeLoreItems(collection.Items)
		if !collection.LegacyMigrated {
			legacyItems := s.legacyItems()
			if len(legacyItems) > 0 {
				collection.Items = mergeLoreItems(collection.Items, legacyItems)
				collection.LegacyMigrated = true
				if err := s.save(collection); err != nil {
					return LoreCollection{}, err
				}
			}
		}
		return collection, nil
	}
	if !os.IsNotExist(err) {
		return LoreCollection{}, err
	}
	legacyItems := s.legacyItems()
	collection := LoreCollection{
		Version:        loreItemsVersion,
		LegacyMigrated: len(legacyItems) > 0,
		Items:          legacyItems,
	}
	if err := s.save(collection); err != nil {
		return LoreCollection{}, err
	}
	return collection, nil
}

func (s *LoreStore) save(collection LoreCollection) error {
	collection.Version = loreItemsVersion
	collection.Items = normalizeLoreItems(collection.Items)
	if err := os.MkdirAll(filepath.Dir(s.itemsPath()), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(collection, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.itemsPath(), append(data, '\n'), 0o644)
}

func (s *LoreStore) legacyItems() []LoreItem {
	now := time.Now().Format(time.RFC3339)
	candidates := []struct {
		id         string
		itemType   string
		name       string
		importance string
		file       string
	}{
		{"characters", "character", "角色设定", "major", filepath.Join(s.workspace, "setting", "characters.md")},
		{"world-building", "world", "世界观设定", "major", filepath.Join(s.workspace, "setting", "world-building.md")},
	}
	items := make([]LoreItem, 0, len(candidates))
	for _, candidate := range candidates {
		data, err := os.ReadFile(candidate.file)
		if err != nil || strings.TrimSpace(string(data)) == "" {
			continue
		}
		items = append(items, normalizeLoreItem(LoreItem{
			ID:         candidate.id,
			Type:       candidate.itemType,
			Name:       candidate.name,
			Importance: candidate.importance,
			Content:    string(data),
			CreatedAt:  now,
			UpdatedAt:  now,
		}))
	}
	return items
}

func (s *LoreStore) itemsPath() string {
	return filepath.Join(s.workspace, ".nova", "lore", "items.json")
}

func (s *LoreStore) hasItem(items []LoreItem, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

func normalizeLoreItems(items []LoreItem) []LoreItem {
	normalized := make([]LoreItem, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		item = normalizeLoreItem(item)
		if item.ID == "" || seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		normalized = append(normalized, item)
	}
	return normalized
}

func mergeLoreItems(current, incoming []LoreItem) []LoreItem {
	result := append([]LoreItem(nil), current...)
	seen := map[string]bool{}
	for _, item := range current {
		seen[item.ID] = true
	}
	for _, item := range incoming {
		if item.ID == "" || seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		result = append(result, item)
	}
	return normalizeLoreItems(result)
}

func normalizeLoreItem(item LoreItem) LoreItem {
	item.ID = normalizeLoreID(item.ID)
	item.Type = normalizeLoreType(item.Type)
	item.Name = strings.TrimSpace(item.Name)
	item.Importance = normalizeLoreImportance(item.Importance)
	item.Content = strings.TrimSpace(item.Content)
	item.Tags = normalizeLoreTags(item.Tags)
	return item
}

func normalizeLoreID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	var sb strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func normalizeLoreType(t string) string {
	switch strings.TrimSpace(t) {
	case "character", "world", "location", "faction", "rule", "item", "other":
		return strings.TrimSpace(t)
	default:
		return "other"
	}
}

func normalizeLoreImportance(v string) string {
	switch strings.TrimSpace(v) {
	case "major", "important", "minor":
		return strings.TrimSpace(v)
	default:
		return "important"
	}
}

func normalizeLoreTags(tags []string) []string {
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

func newLoreID(itemType string) string {
	itemType = normalizeLoreType(itemType)
	return fmt.Sprintf("%s-%d", itemType, time.Now().UTC().UnixNano())
}

func loreImportanceRank(v string) int {
	switch normalizeLoreImportance(v) {
	case "major":
		return 0
	case "important":
		return 1
	default:
		return 2
	}
}

func loreTypeLabel(t string) string {
	switch normalizeLoreType(t) {
	case "character":
		return "角色"
	case "world":
		return "世界观"
	case "location":
		return "地点"
	case "faction":
		return "势力"
	case "rule":
		return "规则"
	case "item":
		return "物品"
	default:
		return "其他"
	}
}

func loreImportanceLabel(v string) string {
	switch normalizeLoreImportance(v) {
	case "major":
		return "主要"
	case "important":
		return "重要"
	default:
		return "次要"
	}
}
