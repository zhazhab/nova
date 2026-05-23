package book

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	gitCommandTimeout = 10 * time.Second
	gitOutputLimit    = 200 * 1024
	// DefaultAutoCommitLineThreshold 是「对话前自动 commit」默认触发阈值（add+del 行数）。
	DefaultAutoCommitLineThreshold = 50
)

var (
	ErrGitNotInstalled = errors.New("未找到 git 命令，请先安装 Git")
	ErrGitNotInit      = errors.New("当前书籍尚未初始化 Git 仓库")
	ErrGitDirty        = errors.New("当前工作区有未提交变更，请先提交后再回滚")
	ErrGitClean        = errors.New("当前工作区没有可暂存的未提交变更")
)

// GitService 管理当前书籍 workspace 的本地 Git 操作。
type GitService struct {
	workspace string
}

// GitStatus 表示当前 Git 仓库状态。
type GitStatus struct {
	Initialized bool        `json:"initialized"`
	Branch      string      `json:"branch"`
	Clean       bool        `json:"clean"`
	Changes     []GitChange `json:"changes"`
}

// GitChange 表示一个工作区变更文件。
type GitChange struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

// GitCommit 表示一条提交历史。
type GitCommit struct {
	Hash      string `json:"hash"`
	ShortHash string `json:"short_hash"`
	Author    string `json:"author"`
	Date      string `json:"date"`
	Subject   string `json:"subject"`
}

// GitCommandResult 表示一次 Git 命令执行结果。
type GitCommandResult struct {
	Command string     `json:"command"`
	Output  string     `json:"output"`
	Status  *GitStatus `json:"status,omitempty"`
}

// NewGitService 创建 Git 服务。
func NewGitService(workspace string) *GitService {
	return &GitService{workspace: workspace}
}

// Status 返回当前仓库状态。未初始化时返回 Initialized=false。
func (s *GitService) Status(ctx context.Context) (GitStatus, error) {
	if err := ensureGitInstalled(); err != nil {
		return GitStatus{}, err
	}
	if !s.initialized() {
		return GitStatus{Initialized: false, Clean: true, Changes: []GitChange{}}, nil
	}

	output, err := s.runGit(ctx, "status", "--short", "--branch", "--untracked-files=all")
	if err != nil {
		return GitStatus{}, err
	}
	return parseGitStatus(output), nil
}

// History 返回最近提交历史。
func (s *GitService) History(ctx context.Context, limit int) ([]GitCommit, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if err := s.ensureReady(); err != nil {
		return nil, err
	}

	format := "%H%x1f%h%x1f%an%x1f%ad%x1f%s"
	output, err := s.runGit(ctx, "log", "-n", strconv.Itoa(limit), "--date=iso", "--pretty=format:"+format)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	commits := make([]GitCommit, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\x1f", 5)
		if len(parts) != 5 {
			continue
		}
		commits = append(commits, GitCommit{
			Hash:      parts[0],
			ShortHash: parts[1],
			Author:    parts[2],
			Date:      parts[3],
			Subject:   parts[4],
		})
	}
	return commits, nil
}

// Diff 返回当前工作区 diff。
func (s *GitService) Diff(ctx context.Context, path string) (string, error) {
	if err := s.ensureReady(); err != nil {
		return "", err
	}
	args := []string{"diff"}
	if strings.TrimSpace(path) != "" {
		args = append(args, "--", path)
	}
	return s.runGit(ctx, args...)
}

// Init 初始化当前书籍 workspace 的 Git 仓库。
func (s *GitService) Init(ctx context.Context) (GitCommandResult, error) {
	if err := ensureGitInstalled(); err != nil {
		return GitCommandResult{}, err
	}
	output, err := s.runGit(ctx, "init")
	if err != nil {
		return GitCommandResult{}, err
	}
	status, statusErr := s.Status(ctx)
	result := GitCommandResult{
		Command: "git init",
		Output:  output,
	}
	if statusErr == nil {
		result.Status = &status
	}
	return result, nil
}

// CreateVersion 创建一个书籍版本，内部执行 add -A 和 commit。
func (s *GitService) CreateVersion(ctx context.Context, message string) (GitCommandResult, error) {
	if err := s.ensureReady(); err != nil {
		return GitCommandResult{}, err
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return GitCommandResult{}, errors.New("版本说明不能为空")
	}

	addOutput, err := s.runGit(ctx, "add", "-A")
	if err != nil {
		return GitCommandResult{}, err
	}
	commitOutput, err := s.runGit(ctx, "commit", "-m", message)
	if err != nil {
		if strings.Contains(err.Error(), "nothing to commit") || strings.Contains(err.Error(), "无文件要提交") {
			return GitCommandResult{}, errors.New("没有可提交的变更")
		}
		return GitCommandResult{}, err
	}

	status, statusErr := s.Status(ctx)
	result := GitCommandResult{
		Command: fmt.Sprintf("git add -A; git commit -m %q", message),
		Output:  strings.TrimSpace("$ git add -A\n" + strings.TrimSpace(addOutput) + "\n\n$ git commit -m " + strconv.Quote(message) + "\n" + strings.TrimSpace(commitOutput)),
	}
	if statusErr == nil {
		result.Status = &status
	}
	return result, nil
}

// AutoCommit 是「对话前自动提交」入口：仓库未初始化或工作区干净则跳过；
// 当未提交变更累计行数 ≥ threshold 时执行一次 add -A + commit，并在返回值中
// 用 Skipped 标记是否真的提交了。threshold ≤ 0 时使用 DefaultAutoCommitLineThreshold。
type AutoCommitResult struct {
	Skipped bool   // true 表示未提交（仓库未初始化 / 工作区干净 / 行数不足阈值）
	Reason  string // Skipped=true 时给出原因
	Lines   int    // 此次累计的 add+del 行数
	Commit  string // 实际提交的短哈希（Skipped=false 时填充）
}

func (s *GitService) AutoCommit(ctx context.Context, threshold int) (AutoCommitResult, error) {
	if threshold <= 0 {
		threshold = DefaultAutoCommitLineThreshold
	}
	if err := ensureGitInstalled(); err != nil {
		return AutoCommitResult{}, err
	}
	if !s.initialized() {
		return AutoCommitResult{Skipped: true, Reason: "仓库未初始化"}, nil
	}
	status, err := s.Status(ctx)
	if err != nil {
		return AutoCommitResult{}, err
	}
	if status.Clean {
		return AutoCommitResult{Skipped: true, Reason: "工作区干净"}, nil
	}

	lines, err := s.pendingChangeLines(ctx)
	if err != nil {
		return AutoCommitResult{}, err
	}
	if lines < threshold {
		return AutoCommitResult{Skipped: true, Reason: fmt.Sprintf("变更 %d 行 < 阈值 %d", lines, threshold), Lines: lines}, nil
	}

	message := fmt.Sprintf("Nova 自动快照：对话前 %s（%d 行变更）", time.Now().Format("2006-01-02 15:04:05"), lines)
	if _, err := s.runGit(ctx, "add", "-A"); err != nil {
		return AutoCommitResult{}, err
	}
	if _, err := s.runGit(ctx, "commit", "-m", message); err != nil {
		return AutoCommitResult{}, err
	}
	short, err := s.runGit(ctx, "rev-parse", "--short", "HEAD")
	if err != nil {
		return AutoCommitResult{Lines: lines}, nil
	}
	return AutoCommitResult{Lines: lines, Commit: strings.TrimSpace(short)}, nil
}

// pendingChangeLines 计算当前未提交（含 staged、unstaged、untracked）的累计 add+del 行数。
func (s *GitService) pendingChangeLines(ctx context.Context) (int, error) {
	total := 0
	tracked, err := s.runGit(ctx, "diff", "HEAD", "--numstat")
	if err != nil {
		// 仓库尚无 commit 时 HEAD 不存在；退化为对所有已暂存 + 未暂存的统计
		tracked, err = s.runGit(ctx, "diff", "--numstat")
		if err != nil {
			return 0, err
		}
	}
	total += sumNumstat(tracked)

	// untracked 文件不会出现在 git diff 中，单独按整文件行数累计。
	untrackedOut, err := s.runGit(ctx, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return total, nil
	}
	for _, name := range strings.Split(strings.TrimSpace(untrackedOut), "\n") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		path := filepath.Join(s.workspace, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		total += countLines(data)
	}
	return total, nil
}

func sumNumstat(output string) int {
	total := 0
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		add, err1 := strconv.Atoi(fields[0])
		del, err2 := strconv.Atoi(fields[1])
		if err1 != nil || err2 != nil {
			// 二进制文件 numstat 显示为 "-"，忽略。
			continue
		}
		total += add + del
	}
	return total
}

func countLines(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	lines := bytes.Count(data, []byte{'\n'})
	if data[len(data)-1] != '\n' {
		lines++
	}
	return lines
}

// Rollback 将整本书回滚到指定 commit。回滚前要求工作区干净。
func (s *GitService) Rollback(ctx context.Context, hash string) (GitCommandResult, error) {
	if err := s.ensureReady(); err != nil {
		return GitCommandResult{}, err
	}
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return GitCommandResult{}, errors.New("回滚版本不能为空")
	}
	status, err := s.Status(ctx)
	if err != nil {
		return GitCommandResult{}, err
	}
	if !status.Clean {
		return GitCommandResult{}, ErrGitDirty
	}

	output, err := s.runGit(ctx, "reset", "--hard", hash)
	if err != nil {
		return GitCommandResult{}, err
	}
	nextStatus, statusErr := s.Status(ctx)
	result := GitCommandResult{
		Command: "git reset --hard " + hash,
		Output:  output,
	}
	if statusErr == nil {
		result.Status = &nextStatus
	}
	return result, nil
}

// Stash 暂存当前未提交内容，包含未跟踪文件。
func (s *GitService) Stash(ctx context.Context) (GitCommandResult, error) {
	if err := s.ensureReady(); err != nil {
		return GitCommandResult{}, err
	}
	status, err := s.Status(ctx)
	if err != nil {
		return GitCommandResult{}, err
	}
	if status.Clean {
		return GitCommandResult{}, ErrGitClean
	}

	output, err := s.runGit(ctx, "stash", "push", "-u", "-m", "Nova stash")
	if err != nil {
		return GitCommandResult{}, err
	}
	nextStatus, statusErr := s.Status(ctx)
	result := GitCommandResult{
		Command: "git stash push -u -m \"Nova stash\"",
		Output:  output,
	}
	if statusErr == nil {
		result.Status = &nextStatus
	}
	return result, nil
}

// PopStash 恢复最近一次暂存的未提交内容。
func (s *GitService) PopStash(ctx context.Context) (GitCommandResult, error) {
	if err := s.ensureReady(); err != nil {
		return GitCommandResult{}, err
	}
	output, err := s.runGit(ctx, "stash", "pop")
	if err != nil {
		return GitCommandResult{}, err
	}
	nextStatus, statusErr := s.Status(ctx)
	result := GitCommandResult{
		Command: "git stash pop",
		Output:  output,
	}
	if statusErr == nil {
		result.Status = &nextStatus
	}
	return result, nil
}

// RunCommand 执行白名单 Git 命令。
func (s *GitService) RunCommand(ctx context.Context, input string) (GitCommandResult, error) {
	commands, err := parseGitCommandSequence(input)
	if err != nil {
		return GitCommandResult{}, err
	}

	var outputs []string
	var executed []string
	for _, args := range commands {
		if len(args) == 0 {
			return GitCommandResult{}, errors.New("Git 命令不能为空")
		}
		if args[0] != "init" {
			if err := s.ensureReady(); err != nil {
				return GitCommandResult{}, err
			}
		} else if err := ensureGitInstalled(); err != nil {
			return GitCommandResult{}, err
		}

		commandText := strings.Join(append([]string{"git"}, args...), " ")
		output, err := s.runGit(ctx, args...)
		if err != nil {
			return GitCommandResult{}, err
		}
		executed = append(executed, commandText)
		outputs = append(outputs, "$ "+commandText+"\n"+strings.TrimSpace(output))
	}

	status, statusErr := s.Status(ctx)
	result := GitCommandResult{
		Command: strings.Join(executed, "; "),
		Output:  strings.TrimSpace(strings.Join(outputs, "\n\n")),
	}
	if statusErr == nil {
		result.Status = &status
	}
	return result, nil
}

func (s *GitService) ensureReady() error {
	if err := ensureGitInstalled(); err != nil {
		return err
	}
	if !s.initialized() {
		return ErrGitNotInit
	}
	return nil
}

func (s *GitService) initialized() bool {
	info, err := os.Stat(filepath.Join(s.workspace, ".git"))
	return err == nil && info.IsDir()
}

func (s *GitService) runGit(ctx context.Context, args ...string) (string, error) {
	runCtx, cancel := context.WithTimeout(ctx, gitCommandTimeout)
	defer cancel()

	baseArgs := []string{"-c", "user.name=Nova", "-c", "user.email=nova@example.invalid", "-c", "core.quotepath=false"}
	cmd := exec.CommandContext(runCtx, "git", append(baseArgs, args...)...)
	cmd.Dir = s.workspace
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	output := limitOutput(out.String())
	if runCtx.Err() == context.DeadlineExceeded {
		return output, errors.New("Git 命令执行超时")
	}
	if err != nil {
		if output == "" {
			output = err.Error()
		}
		return output, errors.New(strings.TrimSpace(output))
	}
	return output, nil
}

func ensureGitInstalled() error {
	if _, err := exec.LookPath("git"); err != nil {
		return ErrGitNotInstalled
	}
	return nil
}

func parseGitCommandSequence(input string) ([][]string, error) {
	parts, err := splitGitCommandSequence(input)
	if err != nil {
		return nil, err
	}
	if len(parts) == 0 {
		return nil, errors.New("Git 命令不能为空")
	}

	commands := make([][]string, 0, len(parts))
	for _, part := range parts {
		args, err := parseGitCommand(part)
		if err != nil {
			return nil, err
		}
		commands = append(commands, args)
	}
	return commands, nil
}

func parseGitCommand(input string) ([]string, error) {
	if hasShellMeta(input) {
		return nil, errors.New("只支持受限 Git 命令，不允许 shell 语法")
	}
	tokens, err := splitGitCommand(input)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return nil, errors.New("Git 命令不能为空")
	}
	if tokens[0] == "git" {
		tokens = tokens[1:]
	}
	if len(tokens) == 0 {
		return nil, errors.New("Git 命令不能为空")
	}

	switch tokens[0] {
	case "init":
		if len(tokens) != 1 {
			return nil, errors.New("init 不支持额外参数")
		}
		return tokens, nil
	case "status":
		if len(tokens) != 1 {
			return nil, errors.New("status 不支持额外参数")
		}
		return []string{"status", "--short", "--branch"}, nil
	case "add":
		if len(tokens) < 2 {
			return nil, errors.New("add 需要指定文件路径，例如 add .")
		}
		return tokens, nil
	case "commit":
		if !hasCommitMessage(tokens) {
			return nil, errors.New("commit 必须包含非空 -m 提交说明")
		}
		return tokens, nil
	case "diff":
		if len(tokens) > 2 {
			return nil, errors.New("diff 仅支持可选单个路径")
		}
		if len(tokens) == 2 {
			return []string{"diff", "--", tokens[1]}, nil
		}
		return tokens, nil
	case "log":
		return []string{"log", "--oneline", "--decorate", "-n", "20"}, nil
	case "history":
		return []string{"log", "--oneline", "--decorate", "-n", "20"}, nil
	case "reset":
		return parseResetCommand(tokens)
	default:
		return nil, fmt.Errorf("不支持的 Git 命令: %s。允许: init/status/add/commit/diff/log/history/reset --soft|--mixed", tokens[0])
	}
}

func parseResetCommand(tokens []string) ([]string, error) {
	if len(tokens) < 2 || len(tokens) > 3 {
		return nil, errors.New("reset 用法: reset [--soft|--mixed] <rev>")
	}
	mode := "--mixed"
	rev := ""
	if strings.HasPrefix(tokens[1], "--") {
		mode = tokens[1]
		if len(tokens) != 3 {
			return nil, errors.New("reset 需要指定目标版本")
		}
		rev = tokens[2]
	} else {
		rev = tokens[1]
	}
	if mode != "--soft" && mode != "--mixed" {
		return nil, errors.New("首版 reset 仅支持 --soft 或 --mixed")
	}
	return []string{"reset", mode, rev}, nil
}

func splitGitCommandSequence(input string) ([]string, error) {
	var parts []string
	var current strings.Builder
	var quote rune
	escaped := false

	for _, r := range strings.TrimSpace(input) {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			current.WriteRune(r)
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			current.WriteRune(r)
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			current.WriteRune(r)
			continue
		}
		if r == ';' {
			part := strings.TrimSpace(current.String())
			if part == "" {
				return nil, errors.New("分号前后的 Git 命令不能为空")
			}
			parts = append(parts, part)
			current.Reset()
			continue
		}
		current.WriteRune(r)
	}
	if quote != 0 {
		return nil, errors.New("命令引号未闭合")
	}
	part := strings.TrimSpace(current.String())
	if part != "" {
		parts = append(parts, part)
	}
	return parts, nil
}

func splitGitCommand(input string) ([]string, error) {
	var tokens []string
	var current strings.Builder
	var quote rune
	escaped := false

	for _, r := range strings.TrimSpace(input) {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == ' ' || r == '\t' || r == '\n' {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(r)
	}
	if quote != 0 {
		return nil, errors.New("命令引号未闭合")
	}
	if escaped {
		current.WriteRune('\\')
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens, nil
}

func hasShellMeta(input string) bool {
	return strings.ContainsAny(input, "&|`<>")
}

func hasCommitMessage(tokens []string) bool {
	for i := 1; i < len(tokens); i++ {
		if tokens[i] == "-m" || tokens[i] == "--message" {
			return i+1 < len(tokens) && strings.TrimSpace(tokens[i+1]) != ""
		}
		if strings.HasPrefix(tokens[i], "-m") && strings.TrimSpace(strings.TrimPrefix(tokens[i], "-m")) != "" {
			return true
		}
		if strings.HasPrefix(tokens[i], "--message=") && strings.TrimSpace(strings.TrimPrefix(tokens[i], "--message=")) != "" {
			return true
		}
	}
	return false
}

func parseGitStatus(output string) GitStatus {
	status := GitStatus{Initialized: true, Clean: true, Changes: []GitChange{}}
	for _, line := range strings.Split(strings.TrimRight(output, "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, "## ") {
			status.Branch = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			continue
		}
		status.Clean = false
		code := strings.TrimSpace(line[:min(2, len(line))])
		path := strings.TrimSpace(line[min(3, len(line)):])
		status.Changes = append(status.Changes, GitChange{Path: path, Status: code})
	}
	return status
}

func limitOutput(output string) string {
	if len(output) <= gitOutputLimit {
		return output
	}
	return output[:gitOutputLimit] + "\n\n[输出已截断]"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
