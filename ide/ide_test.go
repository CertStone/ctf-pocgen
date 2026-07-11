package ide

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestIsExecutable 验证 isExecutable 的判断逻辑。
func TestIsExecutable(t *testing.T) {
	dir := t.TempDir()

	// 不存在的路径
	if isExecutable(filepath.Join(dir, "nope")) {
		t.Error("不存在的路径不应判定为可执行")
	}

	// 目录不应判定为可执行
	if isExecutable(dir) {
		t.Error("目录不应判定为可执行")
	}

	// 存在的文件
	f := filepath.Join(dir, "idea.sh")
	if err := os.WriteFile(f, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		// Windows：文件存在即可
		if !isExecutable(f) {
			t.Error("Windows 上存在的文件应判定为可执行")
		}
	} else {
		// Unix：无执行位应返回 false
		if isExecutable(f) {
			t.Error("无执行位的文件不应判定为可执行")
		}
		// 加执行位后应返回 true
		if err := os.Chmod(f, 0o755); err != nil {
			t.Fatal(err)
		}
		if !isExecutable(f) {
			t.Error("有执行位的文件应判定为可执行")
		}
	}
}

// TestPathCandidateNames 验证各平台 PATH 查找的命令名。
func TestPathCandidateNames(t *testing.T) {
	names := pathCandidateNames()
	if len(names) == 0 {
		t.Fatal("应返回至少一个候选名")
	}
	if runtime.GOOS == "windows" {
		// Windows 应包含 idea.exe
		found := false
		for _, n := range names {
			if n == "idea.exe" {
				found = true
			}
		}
		if !found {
			t.Error("Windows 候选应包含 idea.exe")
		}
	} else {
		// 类 Unix 第一个应是 idea
		if names[0] != "idea" {
			t.Errorf("类 Unix 首选应为 idea, got %s", names[0])
		}
	}
}

// TestStandaloneCandidates_NoPanic 验证 standaloneCandidates 在没有
// IDEA 安装时不会 panic（扫的目录可能不存在）。
func TestStandaloneCandidates_NoPanic(t *testing.T) {
	// 不应 panic，返回值可以是空切片
	cands := standaloneCandidates()
	// 仅断言它运行完成；具体内容依赖本机环境，不做断言
	_ = cands
}

// TestToolboxScriptsDir_NoPanic 验证 toolboxScriptsDir 不 panic。
func TestToolboxScriptsDir_NoPanic(t *testing.T) {
	_ = toolboxScriptsDir()
}

// TestFind_NoPanicAtLeast 验证 Find 在无 IDEA 环境下不 panic，
// 未找到时返回 ErrNotFound。
func TestFind_NoPanicAtLeast(t *testing.T) {
	// 本机可能装了 IDEA，Find 可能成功也可能失败，重点是不 panic
	_, _, err := Find()
	if err != nil && err != ErrNotFound {
		t.Errorf("未找到时应返回 ErrNotFound, got %v", err)
	}
}
