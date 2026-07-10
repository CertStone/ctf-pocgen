package tui

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// TestBuildBrowserItems 验证目录扫描只列出可处理文件与子目录，跳过隐藏项。
func TestBuildBrowserItems(t *testing.T) {
	dir := t.TempDir()
	root := dir
	// 创建：1 个 jar、1 个普通文件（不应出现）、1 个子目录、1 个隐藏文件
	os.WriteFile(filepath.Join(dir, "a.jar"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("x"), 0o644)
	os.Mkdir(filepath.Join(dir, "sub"), 0o755)
	os.WriteFile(filepath.Join(dir, ".hidden.jar"), []byte("x"), 0o644)

	items := buildBrowserItems(dir, root)
	names := []string{}
	for _, it := range items {
		names = append(names, it.(fsItem).name)
	}
	if !slices.Contains(names, "a.jar") {
		t.Errorf("应包含 a.jar, got %v", names)
	}
	if !slices.Contains(names, "sub/") {
		t.Errorf("应包含 sub/, got %v", names)
	}
	if slices.Contains(names, "b.txt") {
		t.Errorf("不应包含普通文件 b.txt, got %v", names)
	}
	if slices.Contains(names, ".hidden.jar") {
		t.Errorf("不应包含隐藏文件, got %v", names)
	}
}

// TestBuildBrowserItems_ParentUp 验证非根目录时有 ".." 返回上级。
func TestBuildBrowserItems_ParentUp(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "child")
	os.Mkdir(sub, 0o755)
	os.WriteFile(filepath.Join(sub, "x.jar"), []byte("x"), 0o644)

	items := buildBrowserItems(sub, root)
	// 第一项应为 ".."
	if len(items) == 0 || items[0].(fsItem).name != ".." {
		t.Errorf("子目录第一项应为 '..', got %v", items)
	}
}

// TestConfirmStateTransitions 验证确认组件的 Yes/No 切换与确认。
func TestConfirmStateTransitions(t *testing.T) {
	m := newConfirm("test?", true) // 默认 Yes
	// Enter 确认默认 Yes
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m2.done || !m2.accepted {
		t.Errorf("默认 Yes 时 Enter 应 accepted=true")
	}

	// 切到 No 后确认
	m = newConfirm("test?", true)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.done || m.accepted {
		t.Errorf("切到 No 后 Enter 应 accepted=false")
	}

	// Y 直选
	m = newConfirm("test?", false)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if !m.done || !m.accepted {
		t.Errorf("Y 应 accepted=true")
	}
}

// TestLocationChoice 验证位置选择默认第一项与切换。
func TestLocationChoice(t *testing.T) {
	m := newLocation(
		locationChoice{Label: "A", Path: "/a"},
		locationChoice{Label: "B", Path: "/b"},
	)
	if m.Chosen() != "/a" {
		t.Errorf("默认应选第一项 /a, got %s", m.Chosen())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.Chosen() != "/b" {
		t.Errorf("切换后应选 /b, got %s", m.Chosen())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Chosen() != "/a" {
		t.Errorf("再切回应选 /a, got %s", m.Chosen())
	}
}

// TestStateMachine_FullFlow 用临时目录构造一个 jar，模拟完整 TUI 状态机：
// 浏览→(自动选中第一个)→确认 Yes→同目录→确认更多 No→结束，断言 selected 非空。
func TestStateMachine_FullFlow(t *testing.T) {
	dir := t.TempDir()
	jarPath := filepath.Join(dir, "app.jar")
	os.WriteFile(jarPath, []byte("PK\x03\x04fake"), 0o644)

	m := New(dir)
	// 让列表选中 jar 项（items 中 a.jar 可能不是第一项，手动找）
	for i, it := range m.browser.Items() {
		if fi, ok := it.(fsItem); ok && fi.kind == kindFile {
			m.browser.Select(i)
			break
		}
	}
	// 也可断言列表至少有一个文件项
	if !hasFileItem(m.browser) {
		t.Fatal("列表应至少有一个文件项")
	}

	// Enter 选中文件 → 进入确认状态
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m2.(Model)
	if m.state != stateConfirmFile {
		t.Fatalf("应进入 stateConfirmFile, got %d", m.state)
	}
	// 确认 Yes（默认 Yes，直接 Enter）
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m2.(Model)
	// 文件在 root（同目录），应直接进入 stateConfirmMore
	if m.state != stateConfirmMore {
		t.Fatalf("同目录应进入 stateConfirmMore, got %d", m.state)
	}
	// 确认更多 = No（默认 No，直接 Enter）→ 结束
	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m2.(Model)
	if m.state != stateDone {
		t.Fatalf("应进入 stateDone, got %d", m.state)
	}
	// 应有一个选中项，输出目录=root
	if len(m.Selected()) != 1 {
		t.Fatalf("应选中 1 个文件, got %d", len(m.Selected()))
	}
	if m.Selected()[0].JarPath != jarPath {
		t.Errorf("JarPath 不匹配: got %s", m.Selected()[0].JarPath)
	}
	if m.Selected()[0].OutputDir != dir {
		t.Errorf("OutputDir 应为 root: got %s", m.Selected()[0].OutputDir)
	}
	// 应返回 tea.Quit 命令
	if cmd == nil {
		t.Error("stateDone 应返回 tea.Quit 命令")
	}
}

func hasFileItem(l list.Model) bool {
	for _, it := range l.Items() {
		if fi, ok := it.(fsItem); ok && fi.kind == kindFile {
			return true
		}
	}
	return false
}
