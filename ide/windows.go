//go:build windows

package ide

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// toolboxScriptsDir 返回 Windows 上 Toolbox 生成脚本的默认目录。
func toolboxScriptsDir() string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		return ""
	}
	return filepath.Join(localAppData, "JetBrains", "Toolbox", "scripts")
}

// standaloneCandidates 发现 standalone 安装的 IDEA 启动器路径。
//
// 优先查注册表（最可靠，IDEA 安装时写入安装路径）：
//
//	HKLM\SOFTWARE\JetBrains\IntelliJ IDEA\<build>  默认值 = 安装目录
//
// 再用 glob 兜底（覆盖 Toolbox 持久化、绿色版等无注册表场景）：
//   - C:\Program Files\JetBrains\IntelliJ IDEA*\bin\idea64.exe
//   - C:\Program Files (x86)\JetBrains\IntelliJ IDEA*\bin\idea64.exe
//   - %LOCALAPPDATA%\Programs\IntelliJ IDEA*\bin\idea64.exe（Toolbox 新版）
func standaloneCandidates() []string {
	var exes []string

	// 策略 A：注册表查询（IDEA 安装时写 HKLM，记录准确安装路径）
	exes = append(exes, findByRegistry()...)

	// 策略 B：glob 兜底
	exes = append(exes, findByGlob()...)

	// 去重并排序（多版本时取最后一个=版本号最大）
	exes = dedupSorted(exes)
	return exes
}

// findByRegistry 从注册表 HKLM\SOFTWARE\JetBrains\IntelliJ IDEA 读取安装路径。
// 兼容 64 位系统的 WOW6432Node 重定向。
func findByRegistry() []string {
	// IDEA 可能写在 64 位视图或 32 位视图（取决于安装器位数）
	keys := []string{
		`SOFTWARE\JetBrains\IntelliJ IDEA`,
		`SOFTWARE\WOW6432Node\JetBrains\IntelliJ IDEA`,
	}
	var exes []string
	for _, keyPath := range keys {
		k, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath,
			registry.READ|registry.ENUMERATE_SUB_KEYS)
		if err != nil {
			continue
		}
		// 枚举子键（每个子键名是 build 号，默认值是安装目录）
		subKeys, err := k.ReadSubKeyNames(-1)
		k.Close()
		if err != nil {
			continue
		}
		for _, sk := range subKeys {
			full := keyPath + `\` + sk
			skKey, err := registry.OpenKey(registry.LOCAL_MACHINE, full, registry.QUERY_VALUE)
			if err != nil {
				continue
			}
			installDir, _, err := skKey.GetStringValue("") // 默认值
			skKey.Close()
			if err != nil || installDir == "" {
				continue
			}
			if exe := findExeInDir(installDir); exe != "" {
				exes = append(exes, exe)
			}
		}
	}
	return exes
}

// findByGlob 在常见安装根目录下 glob IntelliJ IDEA*\bin\idea*.exe。
func findByGlob() []string {
	var roots []string
	for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)"} {
		if v := os.Getenv(env); v != "" {
			roots = append(roots, filepath.Join(v, "JetBrains"))
		}
	}
	if lad := os.Getenv("LOCALAPPDATA"); lad != "" {
		roots = append(roots, filepath.Join(lad, "Programs", "JetBrains"))
		// 有些版本直接在 Programs 下，无 JetBrains 一层
		roots = append(roots, filepath.Join(lad, "Programs"))
	}

	var exes []string
	exeNames := []string{"idea64.exe", "idea.exe"}
	for _, jbRoot := range roots {
		entries, err := os.ReadDir(jbRoot)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !strings.HasPrefix(strings.ToLower(e.Name()), "intellij idea") {
				continue
			}
			binDir := filepath.Join(jbRoot, e.Name(), "bin")
			for _, exe := range exeNames {
				p := filepath.Join(binDir, exe)
				if isExecutable(p) {
					exes = append(exes, p)
				}
			}
		}
	}
	return exes
}

// findExeInDir 在安装目录的 bin 子目录里找 idea64.exe / idea.exe。
func findExeInDir(installDir string) string {
	binDir := filepath.Join(installDir, "bin")
	for _, exe := range []string{"idea64.exe", "idea.exe"} {
		p := filepath.Join(binDir, exe)
		if isExecutable(p) {
			return p
		}
	}
	return ""
}

// dedupSorted 去重并排序。
func dedupSorted(s []string) []string {
	seen := make(map[string]bool, len(s))
	out := s[:0]
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

// launch 在 Windows 上异步启动 IDEA 打开项目。
// 用 cmd /c start 让 IDEA 脱离当前进程独立运行，不阻塞。
func launch(ideaPath, projectDir string) error {
	cmd := exec.Command("cmd", "/c", "start", "", ideaPath, projectDir)
	return cmd.Start()
}
