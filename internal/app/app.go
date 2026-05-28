package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"nova/config"
	"nova/internal/agent"
	"nova/internal/book"
	"nova/internal/interactive"
	"nova/internal/prompts"
	"nova/internal/session"
)

const (
	loreAgentSessionID   = "lore-agent"
	tellerAgentSessionID = "teller-agent"
)

type TellerAgentResult struct {
	Message string               `json:"message"`
	Action  string               `json:"action"`
	Teller  interactive.Teller   `json:"teller"`
	Tellers []interactive.Teller `json:"tellers"`
}

// App 管理 Nova 后端运行时依赖和 workspace 热切换。
type App struct {
	cfg *config.Config

	workspace              string
	bookState              *book.State
	bookService            *book.Service
	interactive            *interactive.Store
	sessionStore           *session.Store
	session                *session.Session
	agentRunner            *adk.Runner
	interactiveStoryRunner *adk.Runner
	chatService            *agent.ChatService
	bookRegistry           *BookRegistry
	bookMetaStore          *BookMetaStore
	gitService             *book.GitService
	activeTask             *Task
	activeInteractiveTask  *Task

	mu sync.RWMutex
}

// New 创建应用运行时。当 workspace 为空且没有最近 workspace 时，App 进入“无书籍”状态，
// 等待用户在前端书籍管理页选择或新建书籍后再构建 runtime。
func New(ctx context.Context, cfg *config.Config) (*App, error) {
	registry := NewBookRegistry(cfg.NovaDir)
	bookMetaStore := NewBookMetaStore(cfg.NovaDir)
	workspace := cfg.Workspace
	if workspace == "" && cfg.ResumeLastWorkspace {
		if lastWorkspace := registry.Current(); lastWorkspace != "" {
			workspace = lastWorkspace
		}
	}

	app := &App{
		cfg:           cfg,
		chatService:   agent.NewChatService(),
		bookRegistry:  registry,
		bookMetaStore: bookMetaStore,
	}

	if workspace == "" {
		log.Printf("[app] 启动时未指定 workspace 且无最近书籍，进入无书籍状态，等待用户在前端选择")
		cfg.Workspace = ""
		return app, nil
	}

	runtime, err := buildRuntime(ctx, cfg, workspace)
	if err != nil {
		return nil, err
	}
	cfg.Workspace = runtime.workspace
	_ = registry.Touch(runtime.workspace)

	app.workspace = runtime.workspace
	app.bookState = runtime.bookState
	app.bookService = runtime.bookService
	app.interactive = runtime.interactive
	app.sessionStore = runtime.sessionStore
	app.session = runtime.session
	app.agentRunner = runtime.agentRunner
	app.interactiveStoryRunner = runtime.interactiveStoryRunner
	app.gitService = runtime.gitService
	return app, nil
}

// ErrNoWorkspace 表示当前 App 尚未绑定任何书籍 workspace。
var ErrNoWorkspace = fmt.Errorf("尚未选择书籍工作区")

// HasWorkspace 返回是否已绑定 workspace。
func (a *App) HasWorkspace() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.workspace != ""
}

// Workspace 返回当前 workspace。
func (a *App) Workspace() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.workspace
}

// BookState 返回当前作品状态管理器。
func (a *App) BookState() *book.State {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.bookState
}

// BookService 返回当前作品文件服务。
func (a *App) BookService() *book.Service {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.bookService
}

func (a *App) InteractiveStories() (interactive.Index, error) {
	a.mu.RLock()
	store := a.interactive
	a.mu.RUnlock()
	if store == nil {
		return interactive.Index{}, ErrNoWorkspace
	}
	return store.Index()
}

func (a *App) CreateInteractiveStory(req interactive.CreateStoryRequest) (interactive.StorySummary, error) {
	a.mu.RLock()
	store := a.interactive
	a.mu.RUnlock()
	if store == nil {
		return interactive.StorySummary{}, ErrNoWorkspace
	}
	return store.CreateStory(req)
}

func (a *App) UpdateInteractiveStory(storyID string, req interactive.UpdateStoryRequest) (interactive.StorySummary, error) {
	a.mu.RLock()
	store := a.interactive
	a.mu.RUnlock()
	if store == nil {
		return interactive.StorySummary{}, ErrNoWorkspace
	}
	return store.UpdateStory(storyID, req)
}

func (a *App) DeleteInteractiveStory(storyID string) error {
	a.mu.RLock()
	store := a.interactive
	sessionStore := a.sessionStore
	a.mu.RUnlock()
	if store == nil {
		return ErrNoWorkspace
	}
	if err := store.DeleteStory(storyID); err != nil {
		return err
	}
	if sessionStore != nil {
		return sessionStore.DeleteByPrefix("interactive-story-" + storyID + "-")
	}
	return nil
}

func (a *App) InteractiveSnapshot(storyID, branchID string) (interactive.Snapshot, error) {
	a.mu.RLock()
	store := a.interactive
	a.mu.RUnlock()
	if store == nil {
		return interactive.Snapshot{}, ErrNoWorkspace
	}
	return store.Snapshot(storyID, branchID)
}

func (a *App) CreateInteractiveBranch(storyID string, req interactive.CreateBranchRequest) (interactive.BranchSummary, error) {
	a.mu.RLock()
	store := a.interactive
	a.mu.RUnlock()
	if store == nil {
		return interactive.BranchSummary{}, ErrNoWorkspace
	}
	return store.CreateBranch(storyID, req)
}

func (a *App) SwitchInteractiveBranch(storyID, branchID string) error {
	a.mu.RLock()
	store := a.interactive
	a.mu.RUnlock()
	if store == nil {
		return ErrNoWorkspace
	}
	return store.SwitchBranch(storyID, branchID)
}

func (a *App) DeleteInteractiveBranch(storyID, branchID string) error {
	a.mu.RLock()
	store := a.interactive
	a.mu.RUnlock()
	if store == nil {
		return ErrNoWorkspace
	}
	return store.DeleteBranch(storyID, branchID)
}

func (a *App) InteractiveBranches(storyID string) ([]interactive.BranchSummary, error) {
	a.mu.RLock()
	store := a.interactive
	a.mu.RUnlock()
	if store == nil {
		return nil, ErrNoWorkspace
	}
	return store.Branches(storyID)
}

func (a *App) AppendInteractiveTurn(storyID, branchID, user, narrative string) (interactive.TurnEvent, error) {
	a.mu.RLock()
	store := a.interactive
	a.mu.RUnlock()
	if store == nil {
		return interactive.TurnEvent{}, ErrNoWorkspace
	}
	return store.AppendTurn(storyID, interactive.AppendTurnRequest{
		BranchID:  branchID,
		User:      user,
		Narrative: narrative,
	})
}

func (a *App) LoreItems() ([]book.LoreItem, error) {
	a.mu.RLock()
	state := a.bookState
	a.mu.RUnlock()
	if state == nil {
		return nil, ErrNoWorkspace
	}
	return book.NewLoreStore(state.Workspace()).List()
}

func (a *App) CreateLoreItem(input book.LoreItemInput) (book.LoreItem, error) {
	a.mu.RLock()
	state := a.bookState
	a.mu.RUnlock()
	if state == nil {
		return book.LoreItem{}, ErrNoWorkspace
	}
	return book.NewLoreStore(state.Workspace()).Create(input)
}

func (a *App) UpdateLoreItem(id string, input book.LoreItemInput) (book.LoreItem, error) {
	a.mu.RLock()
	state := a.bookState
	a.mu.RUnlock()
	if state == nil {
		return book.LoreItem{}, ErrNoWorkspace
	}
	return book.NewLoreStore(state.Workspace()).Update(id, input)
}

func (a *App) DeleteLoreItem(id string) error {
	a.mu.RLock()
	state := a.bookState
	a.mu.RUnlock()
	if state == nil {
		return ErrNoWorkspace
	}
	return book.NewLoreStore(state.Workspace()).Delete(id)
}

func (a *App) LoreVersions() ([]book.LoreVersion, error) {
	a.mu.RLock()
	state := a.bookState
	a.mu.RUnlock()
	if state == nil {
		return nil, ErrNoWorkspace
	}
	return book.NewLoreStore(state.Workspace()).Versions()
}

func (a *App) CreateLoreVersion(message string) (book.LoreVersion, error) {
	a.mu.RLock()
	state := a.bookState
	a.mu.RUnlock()
	if state == nil {
		return book.LoreVersion{}, ErrNoWorkspace
	}
	return book.NewLoreStore(state.Workspace()).Snapshot(message)
}

func (a *App) RestoreLoreVersion(id string) ([]book.LoreItem, error) {
	a.mu.RLock()
	state := a.bookState
	a.mu.RUnlock()
	if state == nil {
		return nil, ErrNoWorkspace
	}
	return book.NewLoreStore(state.Workspace()).RestoreVersion(id)
}

func (a *App) RunLoreAgent(ctx context.Context, instruction string, references []string) (book.LoreApplyResult, error) {
	a.mu.RLock()
	state := a.bookState
	cfg := a.cfg
	workspace := a.workspace
	a.mu.RUnlock()
	if state == nil || cfg == nil {
		return book.LoreApplyResult{}, ErrNoWorkspace
	}
	runtimeCfg := *cfg
	runtimeCfg.Workspace = workspace
	store := book.NewLoreStore(state.Workspace())
	items, err := store.List()
	if err != nil {
		return book.LoreApplyResult{}, err
	}
	plan, err := agent.GenerateLoreEditPlan(ctx, &runtimeCfg, instruction, items, references, nil)
	if err != nil {
		return book.LoreApplyResult{}, err
	}
	return store.ApplyOperations(plan.Message, plan.Ops)
}

func (a *App) LoreAgentMessages() ([]session.HistoryEntry, error) {
	a.mu.RLock()
	store := a.sessionStore
	a.mu.RUnlock()
	if store == nil {
		return nil, ErrNoWorkspace
	}
	sess, err := store.GetOrCreate(loreAgentSessionID)
	if err != nil {
		return nil, err
	}
	return sess.History(), nil
}

func (a *App) ClearLoreAgentSession() error {
	a.mu.RLock()
	store := a.sessionStore
	a.mu.RUnlock()
	if store == nil {
		return ErrNoWorkspace
	}
	sess, err := store.GetOrCreate(loreAgentSessionID)
	if err != nil {
		return err
	}
	return sess.Clear()
}

func (a *App) TellerAgentMessages() ([]session.HistoryEntry, error) {
	a.mu.RLock()
	store := a.sessionStore
	a.mu.RUnlock()
	if store == nil {
		return nil, ErrNoWorkspace
	}
	sess, err := store.GetOrCreate(tellerAgentSessionID)
	if err != nil {
		return nil, err
	}
	return sess.History(), nil
}

func (a *App) ClearTellerAgentSession() error {
	a.mu.RLock()
	store := a.sessionStore
	a.mu.RUnlock()
	if store == nil {
		return ErrNoWorkspace
	}
	sess, err := store.GetOrCreate(tellerAgentSessionID)
	if err != nil {
		return err
	}
	return sess.Clear()
}

func (a *App) StartLoreAgentTask(instruction string, references []string) *Task {
	a.mu.RLock()
	state := a.bookState
	cfg := a.cfg
	workspace := a.workspace
	sessionStore := a.sessionStore
	a.mu.RUnlock()
	if state == nil || cfg == nil || sessionStore == nil {
		return nil
	}
	runtimeCfg := *cfg
	runtimeCfg.Workspace = workspace
	workspacePath := state.Workspace()
	sess, err := sessionStore.GetOrCreate(loreAgentSessionID)
	if err != nil {
		log.Printf("[lore-agent-task] load session failed err=%v", err)
		return nil
	}
	history := sess.GetEffectiveMessages()

	return NewTask(func(ctx context.Context, task *Task, emit func(agent.Event)) {
		store := book.NewLoreStore(workspacePath)
		if err := sess.Append(schema.UserMessage(strings.TrimSpace(instruction))); err != nil {
			emit(agent.Event{Type: "error", Data: map[string]string{"message": err.Error()}})
			return
		}
		emitLoreToolCall(emit, "lore-read", "读取资料库", `{"source":"workspace"}`)
		items, err := store.List()
		if err != nil {
			emitLoreError(sess, emit, err)
			return
		}
		emitLoreToolResult(emit, "lore-read", "已读取资料库，共 "+fmt.Sprint(len(items))+" 条资料。")

		emitLoreToolCall(emit, "lore-plan", "生成编辑方案", fmt.Sprintf(`{"references":%d,"history_messages":%d}`, len(references), len(history)))
		plan, err := agent.StreamLoreEditPlan(ctx, &runtimeCfg, instruction, items, references, history, emit)
		if err != nil {
			emitLoreError(sess, emit, err)
			return
		}
		emitLoreToolResult(emit, "lore-plan", fmt.Sprintf("已生成编辑方案，共 %d 个操作。", len(plan.Ops)))

		emitLoreToolCall(emit, "lore-apply", "应用资料库变更", fmt.Sprintf(`{"ops":%d}`, len(plan.Ops)))
		result, err := store.ApplyOperations(plan.Message, plan.Ops)
		if err != nil {
			emitLoreError(sess, emit, err)
			return
		}
		emitLoreToolResult(emit, "lore-apply", loreResultMessage(result))

		_ = sess.Append(schema.AssistantMessage(loreResultMessage(result), nil))
		emit(agent.Event{Type: "lore_result", Data: result})
	})
}

func (a *App) StartTellerAgentTask(instruction string, targetID string) *Task {
	a.mu.RLock()
	cfg := a.cfg
	sessionStore := a.sessionStore
	a.mu.RUnlock()
	if cfg == nil || cfg.NovaDir == "" || sessionStore == nil {
		return nil
	}
	runtimeCfg := *cfg
	library := interactive.NewTellerLibrary(runtimeCfg.NovaDir)
	targetID = strings.TrimSpace(targetID)
	sess, err := sessionStore.GetOrCreate(tellerAgentSessionID)
	if err != nil {
		log.Printf("[teller-agent-task] 加载会话失败 err=%v", err)
		return nil
	}
	history := sess.GetEffectiveMessages()

	return NewTask(func(ctx context.Context, task *Task, emit func(agent.Event)) {
		instruction = strings.TrimSpace(instruction)
		if err := sess.Append(schema.UserMessage(instruction)); err != nil {
			emit(agent.Event{Type: "error", Data: map[string]string{"message": err.Error()}})
			return
		}
		log.Printf("[teller-agent-task] run begin id=%s target_id=%s instruction_len=%d", task.ID(), targetID, len(instruction))
		if err := rejectUnsupportedTellerAgentInstruction(instruction); err != nil {
			emitTellerError(sess, emit, err)
			log.Printf("[teller-agent-task] 拒绝不支持的指令 target_id=%s err=%v", targetID, err)
			return
		}

		emitLoreToolCall(emit, "teller-read", "读取讲述者配置", fmt.Sprintf(`{"target_id":%q}`, targetID))
		tellers, err := library.List()
		if err != nil {
			emitTellerError(sess, emit, err)
			log.Printf("[teller-agent-task] 读取讲述者失败 target_id=%s err=%v", targetID, err)
			return
		}
		if targetID != "" && !tellerExists(tellers, targetID) {
			err := fmt.Errorf("目标讲述者不存在: %s", targetID)
			emitTellerError(sess, emit, err)
			log.Printf("[teller-agent-task] 目标讲述者不存在 target_id=%s", targetID)
			return
		}
		emitLoreToolResult(emit, "teller-read", fmt.Sprintf("已读取讲述者配置，共 %d 个。", len(tellers)))

		mode := "create"
		if targetID != "" {
			mode = "update"
		}
		emitLoreToolCall(emit, "teller-plan", "生成讲述者编辑方案", fmt.Sprintf(`{"mode":%q,"target_id":%q,"history_messages":%d}`, mode, targetID, len(history)))
		plan, err := agent.StreamTellerEditPlan(ctx, &runtimeCfg, instruction, tellers, targetID, history, emit)
		if err != nil {
			emitTellerError(sess, emit, err)
			log.Printf("[teller-agent-task] 生成编辑方案失败 target_id=%s err=%v", targetID, err)
			return
		}
		emitLoreToolResult(emit, "teller-plan", fmt.Sprintf("已生成 %s 方案：%s。", plan.Action, plan.Message))

		emitLoreToolCall(emit, "teller-apply", "应用讲述者变更", fmt.Sprintf(`{"action":%q,"target_id":%q}`, plan.Action, targetID))
		var teller interactive.Teller
		if plan.Action == "create" {
			teller, err = library.Create(plan.Teller)
		} else {
			teller, err = library.Update(targetID, plan.Teller)
		}
		if err != nil {
			emitTellerError(sess, emit, err)
			log.Printf("[teller-agent-task] 应用变更失败 action=%s target_id=%s err=%v", plan.Action, targetID, err)
			return
		}
		nextTellers, err := library.List()
		if err != nil {
			emitTellerError(sess, emit, err)
			log.Printf("[teller-agent-task] 刷新讲述者列表失败 action=%s teller_id=%s err=%v", plan.Action, teller.ID, err)
			return
		}
		result := TellerAgentResult{
			Message: plan.Message,
			Action:  plan.Action,
			Teller:  teller,
			Tellers: nextTellers,
		}
		emitLoreToolResult(emit, "teller-apply", tellerResultMessage(result))
		_ = sess.Append(schema.AssistantMessage(tellerResultMessage(result), nil))
		emit(agent.Event{Type: "teller_result", Data: result})
		log.Printf("[teller-agent-task] run done id=%s action=%s teller_id=%s tellers=%d", task.ID(), plan.Action, teller.ID, len(nextTellers))
	})
}

func emitLoreToolCall(emit func(agent.Event), id, name, args string) {
	emit(agent.Event{Type: "tool_call", Data: map[string]any{
		"id":   id,
		"name": name,
		"args": args,
	}})
}

func emitLoreToolResult(emit func(agent.Event), id, content string) {
	emit(agent.Event{Type: "tool_result", Data: map[string]any{
		"id":      id,
		"content": content,
	}})
}

func emitLoreError(sess *session.Session, emit func(agent.Event), err error) {
	message := err.Error()
	_ = sess.Append(schema.AssistantMessage("执行失败："+message, nil))
	emit(agent.Event{Type: "error", Data: map[string]string{"message": message}})
}

func emitTellerError(sess *session.Session, emit func(agent.Event), err error) {
	message := err.Error()
	_ = sess.Append(schema.AssistantMessage("执行失败："+message, nil))
	emit(agent.Event{Type: "error", Data: map[string]string{"message": message}})
}

func loreResultMessage(result book.LoreApplyResult) string {
	changed := []string{}
	if len(result.Created) > 0 {
		changed = append(changed, fmt.Sprintf("新增 %d", len(result.Created)))
	}
	if len(result.Updated) > 0 {
		changed = append(changed, fmt.Sprintf("更新 %d", len(result.Updated)))
	}
	if len(result.DeletedIDs) > 0 {
		changed = append(changed, fmt.Sprintf("删除 %d", len(result.DeletedIDs)))
	}
	message := strings.TrimSpace(result.Message)
	if message == "" {
		message = "资料库 Agent 已完成"
	}
	if len(changed) > 0 {
		message += "（" + strings.Join(changed, "，") + "）"
	}
	if len(result.Created) > 0 {
		message += "\n新增：" + loreItemNames(result.Created)
	}
	if len(result.Updated) > 0 {
		message += "\n更新：" + loreItemNames(result.Updated)
	}
	if len(result.DeletedIDs) > 0 {
		message += "\n删除：" + strings.Join(result.DeletedIDs, "，")
	}
	return message
}

func tellerResultMessage(result TellerAgentResult) string {
	action := "创建"
	if result.Action == "update" {
		action = "修改"
	}
	message := strings.TrimSpace(result.Message)
	if message == "" {
		message = "讲述者 Agent 已完成"
	}
	if result.Teller.Name != "" {
		message += fmt.Sprintf("（%s：%s）", action, result.Teller.Name)
	}
	return message
}

func tellerExists(tellers []interactive.Teller, id string) bool {
	for _, teller := range tellers {
		if teller.ID == id {
			return true
		}
	}
	return false
}

func rejectUnsupportedTellerAgentInstruction(instruction string) error {
	normalized := strings.ToLower(strings.TrimSpace(instruction))
	deleteWords := []string{"删除", "删掉", "移除", "remove", "delete"}
	tellerWords := []string{"讲述者", "story teller", "teller", "规则包"}
	for _, deleteWord := range deleteWords {
		if !strings.Contains(normalized, deleteWord) {
			continue
		}
		for _, tellerWord := range tellerWords {
			if strings.Contains(normalized, tellerWord) {
				return fmt.Errorf("讲述者 Agent 当前只支持创建或修改单个讲述者，不支持删除")
			}
		}
	}
	return nil
}

func loreItemNames(items []book.LoreItem) string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return strings.Join(names, "，")
}

// StartInteractiveTask 启动互动模式 Agent 任务，输出写回 interactive/story。
func (a *App) StartInteractiveTask(storyID, branchID, message string) *Task {
	a.mu.Lock()
	if a.interactive == nil || a.bookState == nil || a.cfg == nil {
		a.mu.Unlock()
		log.Printf("[interactive-agent-task] 未选择 workspace，无法启动任务")
		return nil
	}
	if a.activeInteractiveTask != nil && a.activeInteractiveTask.Status() == TaskRunning {
		log.Printf("[interactive-agent-task] replace running task id=%s", a.activeInteractiveTask.ID())
		a.activeInteractiveTask.Abort()
	}

	store := a.interactive
	state := a.bookState
	bookService := a.bookService
	chatService := a.chatService
	runtimeCfg := *a.cfg
	workspace := a.workspace
	runtimeCfg.Workspace = workspace
	novaDir := runtimeCfg.NovaDir
	a.mu.Unlock()

	if layered, err := config.LoadLayered(novaDir, workspace); err == nil {
		runtimeCfg.InteractiveReplyTargetChars = appSettingsInt(layered.Effective.InteractiveReplyTargetChars, 1200)
		runtimeCfg.InteractiveMaxTokens = appSettingsInt(layered.Effective.InteractiveMaxTokens, 0)
		log.Printf("[interactive-agent-task] load interactive settings target_chars=%d max_tokens=%d workspace=%s", runtimeCfg.InteractiveReplyTargetChars, runtimeCfg.InteractiveMaxTokens, workspace)
	} else {
		log.Printf("[interactive-agent-task] load interactive settings failed workspace=%s err=%v", workspace, err)
	}

	storyCtx, err := store.StoryContext(storyID, branchID)
	if err != nil {
		log.Printf("[interactive-agent-task] 读取互动故事上下文失败 story_id=%s branch_id=%s err=%v", storyID, branchID, err)
		return nil
	}
	teller := loadInteractiveTeller(novaDir, storyCtx.Meta.StoryTellerID)
	runner, err := buildInteractiveStoryRunner(context.Background(), &runtimeCfg, state, interactiveStoryTellerSystemInput(teller))
	if err != nil {
		log.Printf("[interactive-agent-task] 刷新互动故事 Agent Runner 失败 workspace=%s err=%v", workspace, err)
		return nil
	}
	a.mu.Lock()
	if a.workspace == workspace {
		a.interactiveStoryRunner = runner
	}
	a.mu.Unlock()

	req := agent.ChatRequest{
		Message: message,
	}
	conversation := newInteractiveConversation(store, novaDir, workspace, storyID, branchID, message, runtimeCfg.InteractiveReplyTargetChars)
	task := NewTask(func(ctx context.Context, task *Task, emit func(agent.Event)) {
		log.Printf("[interactive-agent-task] run begin id=%s story_id=%s branch_id=%s message_len=%d", task.ID(), storyID, branchID, len(message))
		chatService.Run(ctx, runner, conversation, bookService, req, emit)
		if turn, stateReady, ok := conversation.LastTurnForState(); ok && !stateReady && ctx.Err() == nil {
			startInteractiveStateTask(&runtimeCfg, conversation, turn)
		}
		log.Printf("[interactive-agent-task] run end id=%s status=%s", task.ID(), task.Status())
	})

	a.mu.Lock()
	a.activeInteractiveTask = task
	a.mu.Unlock()

	return task
}

func (a *App) InteractiveTellers() ([]interactive.Teller, error) {
	if a.cfg == nil || a.cfg.NovaDir == "" {
		return nil, ErrNoWorkspace
	}
	return interactive.NewTellerLibrary(a.cfg.NovaDir).List()
}

func (a *App) InteractiveTeller(id string) (interactive.Teller, error) {
	if a.cfg == nil || a.cfg.NovaDir == "" {
		return interactive.Teller{}, ErrNoWorkspace
	}
	return interactive.NewTellerLibrary(a.cfg.NovaDir).Get(id)
}

func (a *App) CreateInteractiveTeller(teller interactive.Teller) (interactive.Teller, error) {
	if a.cfg == nil || a.cfg.NovaDir == "" {
		return interactive.Teller{}, ErrNoWorkspace
	}
	return interactive.NewTellerLibrary(a.cfg.NovaDir).Create(teller)
}

func (a *App) UpdateInteractiveTeller(id string, teller interactive.Teller) (interactive.Teller, error) {
	if a.cfg == nil || a.cfg.NovaDir == "" {
		return interactive.Teller{}, ErrNoWorkspace
	}
	return interactive.NewTellerLibrary(a.cfg.NovaDir).Update(id, teller)
}

func (a *App) DeleteInteractiveTeller(id string) error {
	if a.cfg == nil || a.cfg.NovaDir == "" {
		return ErrNoWorkspace
	}
	return interactive.NewTellerLibrary(a.cfg.NovaDir).Delete(id)
}

// ActiveInteractiveTask 返回当前互动模式活跃任务（可能为 nil）。
func (a *App) ActiveInteractiveTask() *Task {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.activeInteractiveTask
}

// AbortInteractiveTask 终止当前互动模式活跃任务。
func (a *App) AbortInteractiveTask() {
	a.mu.RLock()
	task := a.activeInteractiveTask
	a.mu.RUnlock()
	if task != nil {
		task.Abort()
	}
}

// Session 返回当前会话。
func (a *App) Session() *session.Session {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.session
}

// Runner 返回当前 Agent Runner。
func (a *App) Runner() *adk.Runner {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.agentRunner
}

// ChatService 返回聊天服务。
func (a *App) ChatService() *agent.ChatService {
	return a.chatService
}

// SwitchWorkspace 切换工作区，并重建状态、会话和 Agent Runner。
func (a *App) SwitchWorkspace(ctx context.Context, path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("路径无效: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("目录不存在: %s", absPath)
	}

	runtime, err := buildRuntime(ctx, a.cfg, absPath)
	if err != nil {
		return "", err
	}

	a.mu.Lock()
	a.workspace = runtime.workspace
	a.bookState = runtime.bookState
	a.bookService = runtime.bookService
	a.interactive = runtime.interactive
	a.sessionStore = runtime.sessionStore
	a.session = runtime.session
	a.agentRunner = runtime.agentRunner
	a.interactiveStoryRunner = runtime.interactiveStoryRunner
	a.gitService = runtime.gitService
	a.cfg.Workspace = runtime.workspace
	a.mu.Unlock()

	_ = a.bookRegistry.Touch(runtime.workspace)
	return runtime.workspace, nil
}

// Books 返回最近打开的书籍工作目录，并从 book.json 填充元信息。
func (a *App) Books() []BookRecord {
	records := a.bookRegistry.List()
	for i := range records {
		meta, err := a.bookMetaStore.Read(records[i].Path)
		if err != nil {
			continue
		}
		if meta.Title != "" {
			records[i].Name = meta.Title
		}
		records[i].Author = meta.Author
	}
	return records
}

// BookInfo 读取指定路径工作区的书籍元信息。
func (a *App) BookInfo(path string) (book.BookMeta, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return book.BookMeta{}, fmt.Errorf("路径无效: %w", err)
	}
	return a.bookMetaStore.Read(absPath)
}

// UpdateBookInfo 更新指定路径工作区的书籍元信息。
func (a *App) UpdateBookInfo(path string, title, author, description string) (book.BookMeta, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return book.BookMeta{}, fmt.Errorf("路径无效: %w", err)
	}
	meta, err := a.bookMetaStore.Read(absPath)
	if err != nil {
		return book.BookMeta{}, err
	}
	if title != "" {
		meta.Title = title
	}
	if author != "" {
		meta.Author = author
	}
	// description 允许设为空字符串（清除简介），所以总是更新
	meta.Description = description
	return a.bookMetaStore.Write(absPath, meta)
}

// RemoveBook 移除书籍记录，不删除磁盘目录。
func (a *App) RemoveBook(path string) error {
	return a.bookRegistry.Remove(path)
}

// CreateBook 创建新书籍工作区：在 parentDir 下创建以 title 命名的子目录，初始化工作区结构和元信息，然后切换到该工作区。
func (a *App) CreateBook(ctx context.Context, parentDir, title, author, description string) (string, book.BookMeta, error) {
	absParent, err := filepath.Abs(parentDir)
	if err != nil {
		return "", book.BookMeta{}, fmt.Errorf("路径无效: %w", err)
	}

	dir := filepath.Join(absParent, title)
	if _, err := os.Stat(dir); err == nil {
		return "", book.BookMeta{}, fmt.Errorf("目录已存在: %s", dir)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", book.BookMeta{}, fmt.Errorf("创建目录失败: %w", err)
	}

	state := book.NewState(dir)
	if err := state.InitWorkspace(); err != nil {
		return "", book.BookMeta{}, fmt.Errorf("初始化工作目录失败: %w", err)
	}

	meta := book.BookMeta{Title: title, Author: author, Description: description}
	meta, err = a.bookMetaStore.Write(dir, meta)
	if err != nil {
		return "", book.BookMeta{}, fmt.Errorf("写入书籍元信息失败: %w", err)
	}

	workspace, switchErr := a.SwitchWorkspace(ctx, dir)
	if switchErr != nil {
		return "", book.BookMeta{}, fmt.Errorf("切换工作区失败: %w", switchErr)
	}

	return workspace, meta, nil
}

// GitStatus 返回当前书籍 workspace 的 Git 状态。
func (a *App) GitStatus(ctx context.Context) (book.GitStatus, error) {
	a.mu.RLock()
	gitService := a.gitService
	a.mu.RUnlock()
	if gitService == nil {
		return book.GitStatus{}, ErrNoWorkspace
	}
	return gitService.Status(ctx)
}

// GitHistory 返回当前书籍 workspace 的 Git 提交历史。
func (a *App) GitHistory(ctx context.Context, limit int) ([]book.GitCommit, error) {
	a.mu.RLock()
	gitService := a.gitService
	a.mu.RUnlock()
	if gitService == nil {
		return nil, ErrNoWorkspace
	}
	return gitService.History(ctx, limit)
}

// GitDiff 返回当前书籍 workspace 的 Git diff。
func (a *App) GitDiff(ctx context.Context, path string) (string, error) {
	a.mu.RLock()
	gitService := a.gitService
	a.mu.RUnlock()
	if gitService == nil {
		return "", ErrNoWorkspace
	}
	return gitService.Diff(ctx, path)
}

// InitGit 初始化当前书籍 workspace 的 Git 仓库。
func (a *App) InitGit(ctx context.Context) (book.GitCommandResult, error) {
	a.mu.RLock()
	gitService := a.gitService
	a.mu.RUnlock()
	if gitService == nil {
		return book.GitCommandResult{}, ErrNoWorkspace
	}
	return gitService.Init(ctx)
}

// CreateGitVersion 创建一个书籍版本。
func (a *App) CreateGitVersion(ctx context.Context, message string) (book.GitCommandResult, error) {
	a.mu.RLock()
	gitService := a.gitService
	a.mu.RUnlock()
	if gitService == nil {
		return book.GitCommandResult{}, ErrNoWorkspace
	}
	return gitService.CreateVersion(ctx, message)
}

// RollbackGitVersion 将整本书回滚到指定版本。
func (a *App) RollbackGitVersion(ctx context.Context, hash string) (book.GitCommandResult, error) {
	a.mu.RLock()
	gitService := a.gitService
	a.mu.RUnlock()
	if gitService == nil {
		return book.GitCommandResult{}, ErrNoWorkspace
	}
	return gitService.Rollback(ctx, hash)
}

// StashGitChanges 暂存当前未提交内容。
func (a *App) StashGitChanges(ctx context.Context) (book.GitCommandResult, error) {
	a.mu.RLock()
	gitService := a.gitService
	a.mu.RUnlock()
	if gitService == nil {
		return book.GitCommandResult{}, ErrNoWorkspace
	}
	return gitService.Stash(ctx)
}

// PopGitStash 恢复最近一次暂存内容。
func (a *App) PopGitStash(ctx context.Context) (book.GitCommandResult, error) {
	a.mu.RLock()
	gitService := a.gitService
	a.mu.RUnlock()
	if gitService == nil {
		return book.GitCommandResult{}, ErrNoWorkspace
	}
	return gitService.PopStash(ctx)
}

// RunGitCommand 执行受限 Git 命令。
func (a *App) RunGitCommand(ctx context.Context, input string) (book.GitCommandResult, error) {
	a.mu.RLock()
	gitService := a.gitService
	a.mu.RUnlock()
	if gitService == nil {
		return book.GitCommandResult{}, ErrNoWorkspace
	}
	return gitService.RunCommand(ctx, input)
}

// ClearSession 为当前会话追加上下文清理标记。
func (a *App) ClearSession() error {
	a.mu.RLock()
	sess := a.session
	a.mu.RUnlock()
	if sess == nil {
		return ErrNoWorkspace
	}
	return sess.Clear()
}

// Sessions 返回当前 workspace 下的会话列表。
func (a *App) Sessions() ([]session.SessionMeta, error) {
	a.mu.RLock()
	store := a.sessionStore
	var activeID string
	if a.session != nil {
		activeID = a.session.ID
	}
	a.mu.RUnlock()
	if store == nil {
		return nil, ErrNoWorkspace
	}
	return store.List(activeID)
}

// CreateSession 新建会话并设置为当前激活会话。
func (a *App) CreateSession(title string) (*session.Session, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sessionStore == nil {
		return nil, ErrNoWorkspace
	}
	a.abortActiveTaskLocked()

	sess, err := a.sessionStore.Create(title)
	if err != nil {
		return nil, err
	}
	if err := a.sessionStore.SetActiveID(sess.ID); err != nil {
		return nil, err
	}
	a.session = sess
	a.activeTask = nil
	return sess, nil
}

// SwitchSession 切换当前激活会话。
func (a *App) SwitchSession(id string) (*session.Session, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sessionStore == nil {
		return nil, ErrNoWorkspace
	}
	a.abortActiveTaskLocked()

	sess, err := a.sessionStore.Get(id)
	if err != nil {
		return nil, err
	}
	if err := a.sessionStore.SetActiveID(sess.ID); err != nil {
		return nil, err
	}
	a.session = sess
	a.activeTask = nil
	return sess, nil
}

// RenameSession 修改会话标题。
func (a *App) RenameSession(id, title string) error {
	a.mu.RLock()
	store := a.sessionStore
	a.mu.RUnlock()
	if store == nil {
		return ErrNoWorkspace
	}
	return store.Rename(id, title)
}

// DeleteSession 删除会话；删除当前会话后自动切换到剩余最近会话。
func (a *App) DeleteSession(id string) (*session.Session, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sessionStore == nil {
		return nil, ErrNoWorkspace
	}

	wasActive := a.session != nil && a.session.ID == id
	if wasActive {
		a.abortActiveTaskLocked()
	}
	if err := a.sessionStore.Delete(id); err != nil {
		return nil, err
	}
	activeID := ""
	if !wasActive && a.session != nil {
		activeID = a.session.ID
	}
	if activeID == "" {
		metas, err := a.sessionStore.List("")
		if err != nil {
			return nil, err
		}
		if len(metas) == 0 {
			sess, createErr := a.sessionStore.GetOrCreate("default")
			if createErr != nil {
				return nil, createErr
			}
			a.session = sess
			activeID = sess.ID
		} else {
			activeID = metas[0].ID
		}
	}
	sess, err := a.sessionStore.GetOrCreate(activeID)
	if err != nil {
		return nil, err
	}
	if err := a.sessionStore.SetActiveID(sess.ID); err != nil {
		return nil, err
	}
	a.session = sess
	if wasActive {
		a.activeTask = nil
	}
	return sess, nil
}

// SessionMessages 返回指定会话或当前会话的完整历史。
func (a *App) SessionMessages(id string) ([]session.HistoryEntry, error) {
	a.mu.RLock()
	store := a.sessionStore
	current := a.session
	a.mu.RUnlock()
	if store == nil {
		return nil, ErrNoWorkspace
	}
	if id == "" {
		if current == nil {
			return nil, ErrNoWorkspace
		}
		return current.History(), nil
	}
	sess, err := store.Get(id)
	if err != nil {
		return nil, err
	}
	return sess.History(), nil
}

// StartTask 启动后台 Agent 任务。如果有正在运行的任务，先终止它。
func (a *App) StartTask(req agent.ChatRequest) *Task {
	a.mu.Lock()
	if a.session == nil || a.bookState == nil || a.cfg == nil {
		a.mu.Unlock()
		log.Printf("[agent-task] 未选择 workspace，无法启动任务")
		return nil
	}
	if a.activeTask != nil && a.activeTask.Status() == TaskRunning {
		log.Printf("[agent-task] replace running task id=%s", a.activeTask.ID())
		a.activeTask.Abort()
	}

	sess := a.session
	state := a.bookState
	bookService := a.bookService
	chatService := a.chatService
	workspace := a.workspace
	gitService := a.gitService
	runtimeCfg := *a.cfg
	runtimeCfg.Workspace = workspace
	novaDir := runtimeCfg.NovaDir
	a.mu.Unlock()

	runner, err := buildAgentRunner(context.Background(), &runtimeCfg, state)
	if err != nil {
		log.Printf("[agent-task] 刷新 Agent Runner 失败 workspace=%s err=%v", workspace, err)
		return nil
	}
	a.mu.Lock()
	if a.workspace == workspace {
		a.agentRunner = runner
	}
	a.mu.Unlock()

	// 对话前自动 commit：默认阈值 50 行；失败不阻断对话，仅记录日志。
	if gitService != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		result, err := gitService.AutoCommit(ctx, book.DefaultAutoCommitLineThreshold)
		cancel()
		if err != nil {
			log.Printf("[auto-commit] workspace=%s 失败 err=%v", workspace, err)
		} else if result.Skipped {
			log.Printf("[auto-commit] workspace=%s 跳过 reason=%q lines=%d", workspace, result.Reason, result.Lines)
		} else {
			log.Printf("[auto-commit] workspace=%s 已提交 commit=%s lines=%d", workspace, result.Commit, result.Lines)
		}
	}

	// 注入工作区配置中的默认风格参考；仅在用户本轮未指定 # 风格时生效。
	if len(req.StyleReferences) == 0 {
		if layered, err := config.LoadLayered(novaDir, workspace); err == nil {
			rules := layered.Effective.StyleRules
			if len(rules) > 0 {
				converted := make([]agent.StyleRule, 0, len(rules))
				for _, r := range rules {
					converted = append(converted, agent.StyleRule{Scene: r.Scene, Styles: r.Styles})
				}
				req.StyleRules = converted
				log.Printf("[agent-task] inject style rules count=%d rules=%q", len(converted), appStyleRuleNames(converted))
			}
		} else {
			log.Printf("[agent-task] load layered settings for style rules failed err=%v", err)
		}
	}

	task := NewTask(func(ctx context.Context, task *Task, emit func(agent.Event)) {
		log.Printf("[agent-task] run begin id=%s message_len=%d references=%d style_references=%d style_rules=%d selections=%d plan_mode=%v", task.ID(), len(req.Message), len(req.References), len(req.StyleReferences), len(req.StyleRules), len(req.Selections), req.PlanMode)
		chatService.Run(ctx, runner, agent.NewSessionConversation(sess), bookService, req, emit)
		log.Printf("[agent-task] run end id=%s status=%s", task.ID(), task.Status())
	})

	a.mu.Lock()
	a.activeTask = task
	a.mu.Unlock()

	return task
}

func appStyleRuleNames(rules []agent.StyleRule) []string {
	names := make([]string, 0, len(rules))
	for _, rule := range rules {
		names = append(names, fmt.Sprintf("%s -> %v", rule.Scene, rule.Styles))
	}
	return names
}

func (a *App) abortActiveTaskLocked() {
	if a.activeTask != nil && a.activeTask.Status() == TaskRunning {
		log.Printf("[agent-task] abort due to session switch/delete id=%s", a.activeTask.ID())
		a.activeTask.Abort()
	}
}

// ActiveTask 返回当前活跃任务（可能为 nil）。
func (a *App) ActiveTask() *Task {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.activeTask
}

// AbortTask 终止当前活跃任务。
func (a *App) AbortTask() {
	a.mu.RLock()
	task := a.activeTask
	a.mu.RUnlock()
	if task != nil {
		task.Abort()
	}
}

// Status 返回当前作品状态摘要。
func (a *App) Status() (bool, string) {
	a.mu.RLock()
	state := a.bookState
	a.mu.RUnlock()
	if state == nil {
		return false, ""
	}
	return state.HasState(), state.CompactContext()
}

// Settings 返回当前生效的分层配置快照。
func (a *App) Settings() (config.LayeredSettings, error) {
	a.mu.RLock()
	workspace := a.workspace
	novaDir := ""
	if a.cfg != nil {
		novaDir = a.cfg.NovaDir
	}
	a.mu.RUnlock()
	return config.LoadLayered(novaDir, workspace)
}

// UpdateUserSettings 持久化用户级配置并返回最新分层快照。
func (a *App) UpdateUserSettings(s config.Settings) (config.LayeredSettings, error) {
	a.mu.RLock()
	novaDir := ""
	if a.cfg != nil {
		novaDir = a.cfg.NovaDir
	}
	a.mu.RUnlock()
	if err := config.WriteSettingsFile(config.UserConfigPath(novaDir), s); err != nil {
		return config.LayeredSettings{}, err
	}
	return a.Settings()
}

// UpdateWorkspaceSettings 持久化当前工作区配置并返回最新分层快照。
func (a *App) UpdateWorkspaceSettings(s config.Settings) (config.LayeredSettings, error) {
	a.mu.RLock()
	workspace := a.workspace
	a.mu.RUnlock()
	if workspace == "" {
		return config.LayeredSettings{}, fmt.Errorf("当前没有打开的工作区")
	}
	if err := config.WriteSettingsFile(config.WorkspaceConfigPath(workspace), s); err != nil {
		return config.LayeredSettings{}, err
	}
	return a.Settings()
}

type runtimeState struct {
	workspace              string
	bookState              *book.State
	bookService            *book.Service
	interactive            *interactive.Store
	sessionStore           *session.Store
	session                *session.Session
	agentRunner            *adk.Runner
	interactiveStoryRunner *adk.Runner
	gitService             *book.GitService
}

func buildRuntime(ctx context.Context, cfg *config.Config, workspace string) (*runtimeState, error) {
	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return nil, fmt.Errorf("解析工作目录失败: %w", err)
	}

	state := book.NewState(absWorkspace)
	if err := state.InitWorkspace(); err != nil {
		return nil, fmt.Errorf("初始化工作目录失败: %w", err)
	}

	store, err := session.NewStore(state.SessionDir())
	if err != nil {
		return nil, fmt.Errorf("创建会话存储失败: %w", err)
	}
	sess, err := store.GetActiveOrCreate()
	if err != nil {
		return nil, fmt.Errorf("创建会话失败: %w", err)
	}

	runtimeCfg := *cfg
	runtimeCfg.Workspace = absWorkspace
	agentRunner, err := buildAgentRunner(ctx, &runtimeCfg, state)
	if err != nil {
		return nil, err
	}
	interactiveStoryRunner, err := buildInteractiveStoryRunner(ctx, &runtimeCfg, state, prompts.InteractiveStorySystemInstructionInput{})
	if err != nil {
		return nil, err
	}

	return &runtimeState{
		workspace:              absWorkspace,
		bookState:              state,
		bookService:            book.NewService(absWorkspace),
		interactive:            interactive.NewStore(absWorkspace),
		sessionStore:           store,
		session:                sess,
		agentRunner:            agentRunner,
		interactiveStoryRunner: interactiveStoryRunner,
		gitService:             book.NewGitService(absWorkspace),
	}, nil
}

func buildAgentRunner(ctx context.Context, cfg *config.Config, state *book.State) (*adk.Runner, error) {
	builtAgent, err := agent.Build(ctx, cfg, state)
	if err != nil {
		return nil, fmt.Errorf("构建 Agent 失败: %w", err)
	}
	return agent.NewRunner(ctx, builtAgent), nil
}

func buildInteractiveStoryRunner(ctx context.Context, cfg *config.Config, state *book.State, teller prompts.InteractiveStorySystemInstructionInput) (*adk.Runner, error) {
	builtAgent, err := agent.BuildInteractiveStory(ctx, cfg, state, teller)
	if err != nil {
		return nil, fmt.Errorf("构建互动故事 Agent 失败: %w", err)
	}
	return agent.NewRunner(ctx, builtAgent), nil
}

func appSettingsInt(v *int, fallback int) int {
	if v == nil || *v <= 0 {
		return fallback
	}
	return *v
}
