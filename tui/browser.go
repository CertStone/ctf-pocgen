package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// entryKind 区分目录与可处理文件。
type entryKind int

const (
	kindUp   entryKind = iota // ".." 返回上级
	kindDir                   // 子目录
	kindFile                  // 可处理的 jar/war/ear 文件
)

// fsItem 实现 list.Item 接口。
type fsItem struct {
	name string // 显示名
	path string // 绝对路径
	kind entryKind
	info string // 描述（大小或类型）
}

func (i fsItem) Title() string       { return i.name }
func (i fsItem) Description() string { return i.info }
func (i fsItem) FilterValue() string { return i.name }

// docStyle 用于列表外框留白。
var docStyle = lipgloss.NewStyle().Margin(1, 2)

// supportedExtensions 是 TUI 列出的可处理文件扩展名。
var supportedExtensions = []string{".jar", ".war", ".ear"}

// buildBrowserItems 扫描 dir，构造可显示项列表：
//   - 若有上级目录（dir != root），加入 ".. 返回上级"
//   - 子目录（隐藏 . 开头的跳过）
//   - .jar/.war/.ear 文件（隐藏 . 开头的跳过）
func buildBrowserItems(dir, root string) []list.Item {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []list.Item{fsItem{name: "(无法读取目录)", kind: kindDir, info: err.Error()}}
	}

	var dirs, files []list.Item
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue // 隐藏文件/目录
		}
		full := filepath.Join(dir, e.Name())
		if e.IsDir() {
			dirs = append(dirs, fsItem{
				name: e.Name() + "/",
				path: full,
				kind: kindDir,
				info: "目录",
			})
		} else if isSupported(e.Name()) {
			info := "文件"
			if fi, err := e.Info(); err == nil {
				info = formatSize(fi.Size())
			}
			files = append(files, fsItem{
				name: e.Name(),
				path: full,
				kind: kindFile,
				info: info,
			})
		}
	}
	// 目录与文件各自字母序
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].(fsItem).name < dirs[j].(fsItem).name
	})
	sort.Slice(files, func(i, j int) bool {
		return files[i].(fsItem).name < files[j].(fsItem).name
	})

	items := append([]list.Item{}, dirs...)
	items = append(items, files...)

	// 若不在根目录，插入 ".." 返回上级
	if dir != root {
		items = append([]list.Item{fsItem{
			name: "..",
			path: filepath.Dir(dir),
			kind: kindUp,
			info: "返回上级目录",
		}}, items...)
	}
	return items
}

func isSupported(name string) bool {
	lower := strings.ToLower(name)
	for _, ext := range supportedExtensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

func formatSize(n int64) string {
	const (
		_  = iota
		KB = 1 << (10 * iota)
		MB
		GB
	)
	switch {
	case n >= GB:
		return formatFloat(float64(n)/float64(GB)) + " GB"
	case n >= MB:
		return formatFloat(float64(n)/float64(MB)) + " MB"
	case n >= KB:
		return formatFloat(float64(n)/float64(KB)) + " KB"
	default:
		return formatFloat(float64(n)) + " B"
	}
}

func formatFloat(f float64) string {
	s := strings.TrimSuffix(strings.TrimRight(
		strings.TrimRight(formatDecimal(f), "0"), "."), "")
	if s == "" {
		s = "0"
	}
	return s
}

// formatDecimal 简单格式化到两位小数（避免引入 strconv 到视图层）。
func formatDecimal(f float64) string {
	// 保留两位小数
	whole := int64(f)
	frac := int64((f - float64(whole)) * 100)
	if frac < 0 {
		frac = -frac
	}
	ws := itoa64(whole)
	fs := itoa64(frac)
	if len(fs) < 2 {
		fs = "0" + fs
	}
	return ws + "." + fs
}

func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// rebuildList 重建 list 的 items（用于进入子目录后刷新）。
func rebuildList(m *list.Model, dir, root string) {
	items := buildBrowserItems(dir, root)
	m.SetItems(items)
	m.Title = "浏览目录: " + dir
	m.Select(0)
}

// 重新导出 KeyMsg 以便其他文件使用（避免重复 import）。
var _ tea.KeyMsg = tea.KeyMsg{}
