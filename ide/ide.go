// Package ide 负责跨平台发现并启动 JetBrains IntelliJ IDEA。
//
// 发现 IDEA 采用多策略回退（从最通用到最兜底）：
//  1. PATH 查找命令行启动器 idea / idea.exe / idea.bat（用户已配置）
//  2. JetBrains Toolbox 自动生成的脚本目录
//  3. standalone 安装目录 glob（Win/Linux）
//  4. macOS app bundle（用 open -a 启动）
//
// 平台差异由 build tags 隔离：windows.go / darwin.go / linux.go 各自实现
// toolboxScriptsDir / standaloneCandidates / launch 这三个平台相关函数。
package ide

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
)

// Find 在系统中查找 IntelliJ IDEA 的启动入口。
//
// 返回 (launchPath, strategy, error)。launchPath 是可直接执行或传给
// 平台启动命令的路径/命令名；strategy 描述命中了哪种发现方式，便于日志。
// 未找到时返回空串与 ErrNotFound。
func Find() (string, string, error) {
	// 策略 1：PATH 查找命令行启动器（最通用，用户已配置）
	if p, ok := findInPath(); ok {
		return p, "PATH", nil
	}
	// 策略 2：Toolbox 脚本目录
	if p, ok := findInToolbox(); ok {
		return p, "Toolbox script", nil
	}
	// 策略 3/4：平台特定（standalone / app bundle）
	if p, strat, ok := findPlatform(); ok {
		return p, strat, nil
	}
	return "", "", ErrNotFound
}

// Open 启动 IDEA 并打开指定项目目录（异步，不阻塞调用方）。
// projectDir 为要打开的 Maven 项目根目录（绝对路径）。
// 内部调用 Find 定位 IDEA，失败返回错误。
func Open(projectDir string) error {
	path, strategy, err := Find()
	if err != nil {
		return err
	}
	_ = strategy
	return launch(path, projectDir)
}

// ErrNotFound 表示系统中未找到 IntelliJ IDEA。
var ErrNotFound = errors.New("未找到 IntelliJ IDEA，请确认已安装；若用 Toolbox 安装，可在 Toolbox 设置里开启 \"Generate shell scripts\"")

// findInPath 在 PATH 中查找 IDEA 命令行启动器。
// 不同平台命令名不同：Windows 是 idea.exe / idea.bat，类 Unix 是 idea。
func findInPath() (string, bool) {
	candidates := pathCandidateNames()
	for _, name := range candidates {
		if p, err := exec.LookPath(name); err == nil {
			return p, true
		}
	}
	return "", false
}

// findInToolbox 在 JetBrains Toolbox 自动生成的脚本目录里查找 idea 脚本。
// Toolbox 默认把脚本放在固定目录（平台相关，见 toolboxScriptsDir）。
func findInToolbox() (string, bool) {
	dir := toolboxScriptsDir()
	if dir == "" {
		return "", false
	}
	// Toolbox 生成的脚本名固定为 idea（Win 上是 idea.bat）
	names := toolboxScriptFileNames()
	for _, name := range names {
		p := filepath.Join(dir, name)
		if isExecutable(p) {
			return p, true
		}
	}
	return "", false
}

// findPlatform 是平台特定发现逻辑的入口（由 windows.go/darwin.go/linux.go 实现）。
// 返回 (path, strategy, found)。
func findPlatform() (string, string, bool) {
	cands := standaloneCandidates()
	if len(cands) == 0 {
		return "", "", false
	}
	// 多版本时取排序后最后一个（版本号最大的）
	sort.Strings(cands)
	return cands[len(cands)-1], "standalone install", true
}

// pathCandidateNames 返回当前平台 PATH 查找时尝试的命令名（平台相关）。
func pathCandidateNames() []string {
	if runtime.GOOS == "windows" {
		return []string{"idea.exe", "idea.bat", "idea64.exe"}
	}
	return []string{"idea"}
}

// toolboxScriptFileNames 返回 Toolbox 脚本目录里要找的文件名（平台相关）。
func toolboxScriptFileNames() []string {
	if runtime.GOOS == "windows" {
		return []string{"idea.bat", "idea.cmd", "idea.exe"}
	}
	return []string{"idea"}
}

// isExecutable 判断路径是否存在且可执行（Windows 上只要有该文件即可）。
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		return !info.IsDir()
	}
	// 类 Unix：检查可执行位
	return !info.IsDir() && (info.Mode()&0o111 != 0)
}
