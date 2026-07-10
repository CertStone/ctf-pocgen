// Package tui 提供无参启动时的交互式文件浏览界面。
//
// 流程：浏览目录 → 选中文件 → 确认处理 → (子目录则选生成位置) → 确认是否继续。
// 收集完成后返回选中的 (jarPath, outputDir) 列表，由调用方批量生成。
package tui

import (
	"path/filepath"
	"strconv"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Selection 表示用户在 TUI 中确认的一个待处理文件。
type Selection struct {
	JarPath   string // 源 jar 绝对路径
	OutputDir string // 生成项目目录所在的目录（绝对）
}

// Model 是 TUI 顶层状态机。
type Model struct {
	state    tuiState
	root     string        // 启动目录（绝对）
	current  string        // 当前浏览目录（绝对）
	browser  list.Model    // 文件/目录列表
	confirm  confirmModel  // 文件确认 / 是否继续
	location locationModel // 生成位置选择
	width    int
	height   int

	pending  fsItem // 当前选中待确认的文件项
	selected []Selection
	quit     bool
}

type tuiState int

const (
	stateBrowse tuiState = iota
	stateConfirmFile
	stateChooseLocation
	stateConfirmMore
	stateDone
)

// New 构造初始 Model，startDir 为 TUI 启动目录（绝对路径）。
func New(startDir string) Model {
	abs, _ := filepath.Abs(startDir)
	items := buildBrowserItems(abs, abs)
	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, 80, 24)
	l.Title = "浏览目录: " + abs
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false) // 文件较少，关闭过滤简化交互
	l.Select(0)
	return Model{
		state:   stateBrowse,
		root:    abs,
		current: abs,
		browser: l,
	}
}

// Selected 返回用户确认的待处理文件列表。
func (m Model) Selected() []Selection { return m.selected }

// Quit 表示用户是否已退出（含 Ctrl+C）。
func (m Model) Quit() bool { return m.quit }

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		h, v := docStyle.GetFrameSize()
		m.browser.SetSize(msg.Width-h, msg.Height-v)
		return m, nil
	case tea.KeyMsg:
		// 全局退出
		if msg.String() == "ctrl+c" {
			m.quit = true
			return m, tea.Quit
		}
		if msg.String() == "q" && m.state == stateBrowse {
			m.quit = true
			return m, tea.Quit
		}
	}

	// 按状态分发
	switch m.state {
	case stateBrowse:
		return m.updateBrowse(msg)
	case stateConfirmFile:
		return m.updateConfirmFile(msg)
	case stateChooseLocation:
		return m.updateChooseLocation(msg)
	case stateConfirmMore:
		return m.updateConfirmMore(msg)
	}
	return m, nil
}

func (m Model) updateBrowse(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		m.browser, cmd = m.browser.Update(msg)
		return m, cmd
	}
	if k.String() == "enter" {
		sel, ok := m.browser.SelectedItem().(fsItem)
		if !ok {
			return m, nil
		}
		switch sel.kind {
		case kindUp, kindDir:
			// 进入子目录或返回上级
			m.current = sel.path
			rebuildList(&m.browser, m.current, m.root)
			return m, nil
		case kindFile:
			// 选中文件 → 确认
			m.pending = sel
			m.confirm = newConfirm("确定处理这个文件吗？\n  "+sel.name, true)
			m.state = stateConfirmFile
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.browser, cmd = m.browser.Update(msg)
	return m, cmd
}

func (m Model) updateConfirmFile(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tea.KeyMsg); !ok {
		return m, nil
	}
	cm, _ := m.confirm.Update(msg)
	m.confirm = cm
	if !m.confirm.done {
		return m, nil
	}
	if !m.confirm.accepted {
		// 取消 → 回到浏览
		m.state = stateBrowse
		return m, nil
	}
	// 接受 → 判断是否需要选生成位置
	// 文件所在目录与启动目录不同时，让用户选生成位置
	if filepath.Dir(m.pending.path) != m.root {
		m.location = newLocation(
			locationChoice{Label: "启动目录", Path: m.root},
			locationChoice{Label: "jar 所在目录", Path: filepath.Dir(m.pending.path)},
		)
		m.state = stateChooseLocation
		return m, nil
	}
	// 同目录 → 直接加入并询问是否继续
	m.selected = append(m.selected, Selection{
		JarPath:   m.pending.path,
		OutputDir: m.root,
	})
	m.confirm = newConfirm("还有其他文件要处理吗？", false)
	m.state = stateConfirmMore
	return m, nil
}

func (m Model) updateChooseLocation(msg tea.Msg) (tea.Model, tea.Cmd) {
	lm, _ := m.location.Update(msg)
	m.location = lm
	if !m.location.done {
		return m, nil
	}
	m.selected = append(m.selected, Selection{
		JarPath:   m.pending.path,
		OutputDir: m.location.Chosen(),
	})
	m.confirm = newConfirm("还有其他文件要处理吗？", false)
	m.state = stateConfirmMore
	return m, nil
}

func (m Model) updateConfirmMore(msg tea.Msg) (tea.Model, tea.Cmd) {
	cm, _ := m.confirm.Update(msg)
	m.confirm = cm
	if !m.confirm.done {
		return m, nil
	}
	if m.confirm.accepted {
		// 继续 → 回浏览（保留已选）
		m.state = stateBrowse
		return m, nil
	}
	// 结束选择
	m.state = stateDone
	return m, tea.Quit
}

func (m Model) View() string {
	switch m.state {
	case stateBrowse:
		return docStyle.Render(m.browser.View())
	case stateConfirmFile:
		return header(m.pending.name) + "\n" + docStyle.Render(m.confirm.View())
	case stateChooseLocation:
		return header(m.pending.name) + "\n" + docStyle.Render(m.location.View())
	case stateConfirmMore:
		summary := summaryOf(m.selected)
		return docStyle.Render(summary + "\n\n" + m.confirm.View())
	case stateDone:
		return "处理完成。"
	}
	return ""
}

func header(name string) string {
	return lipgloss.NewStyle().Bold(true).Render("选中文件: " + name)
}

func summaryOf(sel []Selection) string {
	out := "已选择以下文件待处理：\n"
	for i, s := range sel {
		out += "  " + strconv.Itoa(i+1) + ". " + filepath.Base(s.JarPath)
		out += "  →  " + s.OutputDir + "\n"
	}
	return out
}
