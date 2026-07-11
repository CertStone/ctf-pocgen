//go:build darwin

package ide

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// toolboxScriptsDir 返回 macOS 上 Toolbox 生成脚本的默认目录。
func toolboxScriptsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "Application Support", "JetBrains", "Toolbox", "scripts")
}

// standaloneCandidates 在 macOS 上查找 IDEA app bundle。
// macOS 上 IDEA 是 .app 包，不能直接执行其内部二进制（官方明确说 idea.sh 不适用 Mac），
// 所以这里返回 .app 路径，由 launch 用 open -a 启动。
// 覆盖位置：
//   - /Applications/IntelliJ IDEA*.app
//   - ~/Applications/IntelliJ IDEA*.app（用户级安装）
func standaloneCandidates() []string {
	roots := []string{"/Applications"}
	if home, err := os.UserHomeDir(); err == nil {
		roots = append(roots, filepath.Join(home, "Applications"))
	}
	const prefix = "IntelliJ IDEA"
	var apps []string
	for _, r := range roots {
		entries, err := os.ReadDir(r)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			if !strings.HasPrefix(name, prefix) && !strings.HasPrefix(name, "IntelliJ IDEA") {
				continue
			}
			if !strings.HasSuffix(name, ".app") {
				continue
			}
			p := filepath.Join(r, name)
			if info, err := os.Stat(p); err == nil && info.IsDir() {
				apps = append(apps, p)
			}
		}
	}
	sort.Strings(apps)
	return apps
}

// launch 在 macOS 上异步启动 IDEA。
// macOS 必须用 open -a（直接调 idea.sh 会有 license/环境问题）。
// open 本身就是异步的，立即返回。
func launch(ideaPath, projectDir string) error {
	// ideaPath 可能是 .app 路径（standalone）或命令名（PATH/Toolbox）
	if strings.HasSuffix(ideaPath, ".app") {
		cmd := exec.Command("open", "-a", ideaPath, projectDir)
		return cmd.Start()
	}
	// PATH/Toolbox 找到的是可执行脚本，直接跑
	cmd := exec.Command(ideaPath, projectDir)
	return cmd.Start()
}
