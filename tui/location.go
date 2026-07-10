package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// locationChoice 表示一个生成位置选项。
type locationChoice struct {
	Label string // 显示名，如「启动目录」
	Path  string // 绝对路径
}

// locationModel 让用户在两个生成位置间二选一（默认第一项）。
type locationModel struct {
	choices  []locationChoice
	selected int // 0 或 1
	done     bool
}

func newLocation(c1, c2 locationChoice) locationModel {
	return locationModel{choices: []locationChoice{c1, c2}, selected: 0}
}

// Chosen 返回用户选中的位置路径。
func (m locationModel) Chosen() string {
	return m.choices[m.selected].Path
}

func (m locationModel) Init() tea.Cmd { return nil }

func (m locationModel) Update(msg tea.Msg) (locationModel, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch k.String() {
	case "left", "right", "h", "l", "tab", "up", "down":
		m.selected = 1 - m.selected
	case "1":
		m.selected = 0
	case "2":
		m.selected = 1
	case "enter":
		m.done = true
	case "esc", "ctrl+c":
		m.selected = 0 // 取消则取默认
		m.done = true
	}
	return m, nil
}

func (m locationModel) View() string {
	var b string
	b = "选择项目生成位置：\n\n"
	for i, c := range m.choices {
		style := lipgloss.NewStyle().Padding(0, 2)
		marker := "  "
		if i == m.selected {
			style = style.Background(lipgloss.Color("205")).Foreground(lipgloss.Color("0")).Bold(true)
			marker = "▶ "
		}
		line := marker + c.Label + ": " + c.Path
		b += style.Render(line) + "\n"
	}
	b += "\n" + lipgloss.NewStyle().Faint(true).Render("↑/↓ 或 ←/→ 切换，Enter 确认（默认第一项）")
	return b
}
