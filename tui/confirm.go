package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// confirmModel 是一个极简的 Yes/No 确认组件。
// Bubble Tea 生态没有内置 confirm 组件，这里自写。
// 默认聚焦 Yes（defaultYes 控制）。
type confirmModel struct {
	prompt   string
	yes      bool // 当前聚焦
	accepted bool
	done     bool

	// 复用 list 的渲染尺寸（可选）
	width, height int
}

func newConfirm(prompt string, defaultYes bool) confirmModel {
	return confirmModel{prompt: prompt, yes: defaultYes}
}

func (m confirmModel) Init() tea.Cmd { return nil }

func (m confirmModel) Update(msg tea.Msg) (confirmModel, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch k.String() {
	case "left", "right", "h", "l", "tab":
		m.yes = !m.yes
	case "enter":
		m.accepted = m.yes
		m.done = true
	case "y":
		m.accepted = true
		m.done = true
	case "n":
		m.accepted = false
		m.done = true
	case "esc", "ctrl+c":
		m.accepted = false
		m.done = true
	}
	return m, nil
}

func (m confirmModel) View() string {
	yesStyle := lipgloss.NewStyle().Padding(0, 3)
	noStyle := lipgloss.NewStyle().Padding(0, 3)
	if m.yes {
		yesStyle = yesStyle.Background(lipgloss.Color("205")).Foreground(lipgloss.Color("0")).Bold(true)
	} else {
		noStyle = noStyle.Background(lipgloss.Color("205")).Foreground(lipgloss.Color("0")).Bold(true)
	}
	return m.prompt + "\n\n" + yesStyle.Render("Yes") + "    " + noStyle.Render("No") +
		"\n\n" + lipgloss.NewStyle().Faint(true).Render("←/→ 切换，Enter 确认，Y/N 直选")
}

// 让 confirmModel 也作为 list.Model 的宿主以支持尺寸同步（占位实现）。
var _ list.Model = list.Model{}
