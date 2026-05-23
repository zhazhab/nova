package book

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func newTestGitService(t *testing.T) (*GitService, context.Context) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("当前环境未安装 git")
	}
	root := t.TempDir()
	return NewGitService(root), context.Background()
}

func TestGitStatusUninitialized(t *testing.T) {
	service, ctx := newTestGitService(t)
	status, err := service.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.Initialized {
		t.Fatalf("未初始化仓库应返回 initialized=false: %#v", status)
	}
	if status.Changes == nil {
		t.Fatal("未初始化仓库应返回空 changes 数组而不是 nil")
	}
}

func TestGitInitAndStatus(t *testing.T) {
	service, ctx := newTestGitService(t)
	if _, err := service.RunCommand(ctx, "init"); err != nil {
		t.Fatalf("git init 失败: %v", err)
	}
	status, err := service.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Initialized {
		t.Fatalf("init 后应为 Git 仓库: %#v", status)
	}
	if status.Changes == nil {
		t.Fatal("干净仓库应返回空 changes 数组而不是 nil")
	}
}

func TestGitCommandRejectsDangerousInput(t *testing.T) {
	service, ctx := newTestGitService(t)
	cases := []string{
		"reset --hard HEAD",
		"checkout .",
		"status && rm -rf .",
		"commit",
	}
	for _, input := range cases {
		if _, err := service.RunCommand(ctx, input); err == nil {
			t.Fatalf("危险或非法命令应被拒绝: %s", input)
		}
	}
}

func TestGitAddCommitHistoryAndResetMixed(t *testing.T) {
	service, ctx := newTestGitService(t)
	if _, err := service.RunCommand(ctx, "init"); err != nil {
		t.Fatal(err)
	}

	writeFile(t, service.workspace, "chapters/ch01.md", "第一版\n")
	status, err := service.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.Clean || len(status.Changes) == 0 {
		t.Fatalf("新文件应出现在 status 中: %#v", status)
	}

	if _, err := service.RunCommand(ctx, "add ."); err != nil {
		t.Fatalf("git add 失败: %v", err)
	}
	if _, err := service.RunCommand(ctx, `commit -m "初始化版本"`); err != nil {
		t.Fatalf("第一次 commit 失败: %v", err)
	}

	writeFile(t, service.workspace, "chapters/ch02.md", "第二版\n")
	if _, err := service.RunCommand(ctx, "add ."); err != nil {
		t.Fatalf("第二次 add 失败: %v", err)
	}
	if _, err := service.RunCommand(ctx, `commit -m "第二次版本"`); err != nil {
		t.Fatalf("第二次 commit 失败: %v", err)
	}

	history, err := service.History(ctx, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) < 2 {
		t.Fatalf("提交历史数量不符合预期: %#v", history)
	}

	if _, err := service.RunCommand(ctx, "reset --mixed HEAD~1"); err != nil {
		t.Fatalf("reset --mixed 应可执行: %v", err)
	}
}

func TestGitSequentialAddAAndCommit(t *testing.T) {
	service, ctx := newTestGitService(t)
	if _, err := service.RunCommand(ctx, "init"); err != nil {
		t.Fatal(err)
	}

	writeFile(t, service.workspace, "chapters/ch01.md", "第一章\n")
	result, err := service.RunCommand(ctx, `git add -A; git commit -m "说明"`)
	if err != nil {
		t.Fatalf("连续 add -A 和 commit 应执行成功: %v", err)
	}
	if result.Command != `git add -A; git commit -m 说明` {
		t.Fatalf("执行命令记录不符合预期: %s", result.Command)
	}

	history, err := service.History(ctx, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].Subject != "说明" {
		t.Fatalf("提交历史不符合预期: %#v", history)
	}
}

func TestGitSequentialCommandRejectsUnsupportedPart(t *testing.T) {
	service, ctx := newTestGitService(t)
	if _, err := service.RunCommand(ctx, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.RunCommand(ctx, "status; rm -rf ."); err == nil {
		t.Fatal("连续命令中的非白名单命令应被拒绝")
	}
}

func TestGitCreateVersion(t *testing.T) {
	service, ctx := newTestGitService(t)
	if _, err := service.Init(ctx); err != nil {
		t.Fatal(err)
	}
	writeFile(t, service.workspace, "chapters/ch01.md", "第一章\n")

	if _, err := service.CreateVersion(ctx, "说明"); err != nil {
		t.Fatalf("创建版本失败: %v", err)
	}
	history, err := service.History(ctx, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].Subject != "说明" {
		t.Fatalf("提交历史不符合预期: %#v", history)
	}
	if _, err := service.CreateVersion(ctx, "无变更版本"); err == nil {
		t.Fatal("没有变更时创建版本应失败")
	}
}

func TestGitRollback(t *testing.T) {
	service, ctx := newTestGitService(t)
	if _, err := service.Init(ctx); err != nil {
		t.Fatal(err)
	}

	writeFile(t, service.workspace, "chapters/ch01.md", "第一版\n")
	if _, err := service.CreateVersion(ctx, "第一版"); err != nil {
		t.Fatal(err)
	}
	first, err := service.History(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}

	writeFile(t, service.workspace, "chapters/ch01.md", "第二版\n")
	if _, err := service.CreateVersion(ctx, "第二版"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Rollback(ctx, first[0].Hash); err != nil {
		t.Fatalf("工作区干净时回滚应成功: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(service.workspace, "chapters/ch01.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "第一版\n" {
		t.Fatalf("回滚后文件内容不符合预期: %q", string(content))
	}
}

func TestGitRollbackRejectsDirtyWorkspace(t *testing.T) {
	service, ctx := newTestGitService(t)
	if _, err := service.Init(ctx); err != nil {
		t.Fatal(err)
	}
	writeFile(t, service.workspace, "chapters/ch01.md", "第一版\n")
	if _, err := service.CreateVersion(ctx, "第一版"); err != nil {
		t.Fatal(err)
	}
	history, err := service.History(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, service.workspace, "chapters/ch01.md", "未提交变更\n")

	_, err = service.Rollback(ctx, history[0].Hash)
	if !errors.Is(err, ErrGitDirty) {
		t.Fatalf("工作区有变更时应禁止回滚: %v", err)
	}
}

func TestGitStashAndPop(t *testing.T) {
	service, ctx := newTestGitService(t)
	if _, err := service.Init(ctx); err != nil {
		t.Fatal(err)
	}
	writeFile(t, service.workspace, "chapters/ch01.md", "第一版\n")
	if _, err := service.CreateVersion(ctx, "第一版"); err != nil {
		t.Fatal(err)
	}

	writeFile(t, service.workspace, "chapters/ch01.md", "暂存内容\n")
	writeFile(t, service.workspace, "chapters/ch02.md", "未跟踪文件\n")
	if _, err := service.Stash(ctx); err != nil {
		t.Fatalf("stash 应成功: %v", err)
	}
	status, err := service.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Clean {
		t.Fatalf("stash 后工作区应干净: %#v", status)
	}
	if _, err := os.Stat(filepath.Join(service.workspace, "chapters/ch02.md")); !os.IsNotExist(err) {
		t.Fatalf("stash -u 后未跟踪文件应被暂存: %v", err)
	}

	if _, err := service.PopStash(ctx); err != nil {
		t.Fatalf("stash pop 应成功: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(service.workspace, "chapters/ch01.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "暂存内容\n" {
		t.Fatalf("pop 后文件内容不符合预期: %q", string(content))
	}
	if _, err := os.Stat(filepath.Join(service.workspace, "chapters/ch02.md")); err != nil {
		t.Fatalf("pop 后未跟踪文件应恢复: %v", err)
	}
}

func TestGitStashRejectsCleanWorkspace(t *testing.T) {
	service, ctx := newTestGitService(t)
	if _, err := service.Init(ctx); err != nil {
		t.Fatal(err)
	}
	writeFile(t, service.workspace, "chapters/ch01.md", "第一版\n")
	if _, err := service.CreateVersion(ctx, "第一版"); err != nil {
		t.Fatal(err)
	}
	_, err := service.Stash(ctx)
	if !errors.Is(err, ErrGitClean) {
		t.Fatalf("干净工作区 stash 应返回 ErrGitClean: %v", err)
	}
}

func TestGitStatusKeepsChinesePath(t *testing.T) {
	service, ctx := newTestGitService(t)
	if _, err := service.Init(ctx); err != nil {
		t.Fatal(err)
	}
	path := "setting/萧凡主角风格.md"
	writeFile(t, service.workspace, path, "中文文件名\n")

	status, err := service.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Changes) != 1 {
		t.Fatalf("期望 1 个变更，实际 %#v", status.Changes)
	}
	if status.Changes[0].Path != path {
		t.Fatalf("中文文件名应保持可读原文，实际 %q", status.Changes[0].Path)
	}
}

func TestGitDiff(t *testing.T) {
	service, ctx := newTestGitService(t)
	if _, err := service.RunCommand(ctx, "init"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, service.workspace, "chapters/ch01.md", "第一版\n")
	if _, err := service.RunCommand(ctx, "add ."); err != nil {
		t.Fatal(err)
	}
	if _, err := service.RunCommand(ctx, `commit -m "初始化版本"`); err != nil {
		t.Fatal(err)
	}
	writeFile(t, service.workspace, "chapters/ch01.md", "第一版\n第二行\n")
	diff, err := service.Diff(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if diff == "" {
		t.Fatal("diff 输出不应为空")
	}
}

func TestGitCommandRequiresInitializedRepo(t *testing.T) {
	service, ctx := newTestGitService(t)
	_, err := service.RunCommand(ctx, "status")
	if !errors.Is(err, ErrGitNotInit) {
		t.Fatalf("未初始化仓库应返回 ErrGitNotInit: %v", err)
	}
}

func TestAutoCommitSkipsWhenNotInitialized(t *testing.T) {
	service, ctx := newTestGitService(t)
	result, err := service.AutoCommit(ctx, 5)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Skipped || result.Reason != "仓库未初始化" {
		t.Fatalf("未初始化仓库应跳过 commit: %#v", result)
	}
}

func TestAutoCommitSkipsWhenClean(t *testing.T) {
	service, ctx := newTestGitService(t)
	if _, err := service.RunCommand(ctx, "init"); err != nil {
		t.Fatal(err)
	}
	result, err := service.AutoCommit(ctx, 5)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Skipped || result.Reason != "工作区干净" {
		t.Fatalf("干净工作区应跳过 commit: %#v", result)
	}
}

func TestAutoCommitSkipsWhenBelowThreshold(t *testing.T) {
	service, ctx := newTestGitService(t)
	if _, err := service.RunCommand(ctx, "init"); err != nil {
		t.Fatal(err)
	}
	// 仅 1 行 untracked 文件，阈值 5 -> 跳过
	writeFile(t, service.workspace, "chapters/ch01.md", "一行")
	result, err := service.AutoCommit(ctx, 5)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Skipped {
		t.Fatalf("低于阈值应跳过 commit: %#v", result)
	}
	if result.Lines == 0 {
		t.Fatalf("跳过结果应包含已计算的行数: %#v", result)
	}
}

func TestAutoCommitCommitsWhenAboveThreshold(t *testing.T) {
	service, ctx := newTestGitService(t)
	if _, err := service.RunCommand(ctx, "init"); err != nil {
		t.Fatal(err)
	}
	// untracked 文件 6 行，阈值 5 -> 应触发提交
	content := strings.Repeat("一行\n", 6)
	writeFile(t, service.workspace, "chapters/ch01.md", content)

	result, err := service.AutoCommit(ctx, 5)
	if err != nil {
		t.Fatalf("AutoCommit 失败: %v", err)
	}
	if result.Skipped {
		t.Fatalf("达到阈值应执行 commit: %#v", result)
	}
	if result.Commit == "" {
		t.Fatalf("commit 短哈希应非空: %#v", result)
	}

	status, err := service.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Clean {
		t.Fatalf("AutoCommit 后工作区应干净: %#v", status)
	}
	history, err := service.History(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || !strings.HasPrefix(history[0].Subject, "Nova 自动快照：对话前") {
		t.Fatalf("自动 commit 信息不符合预期: %#v", history)
	}
}

func writeFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	path := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
