package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
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
		KB = 1 << 10
		MB = 1 << 20
		GB = 1 << 30
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

// formatFloat 格式化为最多两位小数（去掉多余的 0 和小数点）。
func formatFloat(f float64) string {
	s := strconv.FormatFloat(f, 'f', 2, 64)
	// 去掉末尾多余的 0 和小数点：12.50 -> 12.5, 12.00 -> 12
	if i := strings.IndexByte(s, '.'); i >= 0 {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s
}

// rebuildList 重建 list 的 items（用于进入子目录后刷新）。
func rebuildList(m *list.Model, dir, root string) {
	items := buildBrowserItems(dir, root)
	m.SetItems(items)
	m.Title = "浏览目录: " + dir
	m.Select(0)
}
