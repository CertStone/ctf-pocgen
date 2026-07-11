//go:build linux

package ide

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

// toolboxScriptsDir 返回 Linux 上 Toolbox 生成脚本的默认目录。
func toolboxScriptsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "JetBrains", "Toolbox", "scripts")
}

// standaloneCandidates 扫描 standalone / Snap 安装的 IDEA 启动器路径。
// 覆盖位置：
//   - ~/.local/share/JetBrains/Toolbox/apps/IDEA*/ch-0/*/bin/idea.sh（Toolbox 持久化安装）
//   - /opt/jetbrains/idea*/bin/idea.sh
//   - /snap/bin/intellij-idea-community / intellij-idea-ultimate（Snap）
func standaloneCandidates() []string {
	var exes []string

	// 1. Snap 安装（命令行入口，最简单）
	for _, name := range []string{"intellij-idea-community", "intellij-idea-ultimate", "intellij-idea"} {
		if p, err := exec.LookPath(name); err == nil {
			exes = append(exes, p)
		}
	}

	// 2. /opt 下的 standalone
	if entries, err := os.ReadDir("/opt"); err == nil {
		for _, e := range entries {
			name := e.Name()
			if !strings.HasPrefix(strings.ToLower(name), "jetbrains/idea") &&
				!strings.HasPrefix(strings.ToLower(name), "idea") {
				continue
			}
			p := filepath.Join("/opt", e.Name(), "bin", "idea.sh")
			if isExecutable(p) {
				exes = append(exes, p)
			}
		}
	}

	// 3. Toolbox 持久化安装目录（apps/IDEA-*/ch-0/<version>/bin/idea.sh）
	home, _ := os.UserHomeDir()
	if home != "" {
		appsDir := filepath.Join(home, ".local", "share", "JetBrains", "Toolbox", "apps")
		exes = append(exes, globToolboxApps(appsDir)...)
	}

	sort.Strings(exes)
	return exes
}

// globToolboxApps 在 Toolbox apps 目录下递归找 idea.sh。
func globToolboxApps(appsDir string) []string {
	// apps 下有 IDEA-C / IDEA-U 等子目录，每个里有 ch-0/<version>/bin/idea.sh
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "IDEA") {
			continue
		}
		// 在 IDEA-C/IDEA-U 下找 idea.sh
		found := walkForIdeaSh(filepath.Join(appsDir, e.Name()))
		out = append(out, found...)
	}
	return out
}

// walkForIdeaSh 在 dir 下（最多 4 层）查找 bin/idea.sh。
func walkForIdeaSh(dir string) []string {
	var out []string
	// 用 filepath.WalkDir 但限制深度避免过深扫描
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			rel := strings.TrimPrefix(path, dir)
			rel = strings.TrimPrefix(rel, string(filepath.Separator))
			// 限制深度：ch-0/<version> 大约 2 层
			if strings.Count(rel, string(filepath.Separator)) > 3 {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == "idea.sh" {
			if isExecutable(path) {
				out = append(out, path)
			}
		}
		return nil
	})
	return out
}

// launch 在 Linux 上异步启动 IDEA。
// 设置独立进程组（Setpgid），本进程退出后 IDEA 不受 SIGHUP 影响。
func launch(ideaPath, projectDir string) error {
	cmd := exec.Command(ideaPath, projectDir)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	// 设置进程组，避免本进程退出影响 IDEA（Setpgid）
	return cmd.Start()
}
