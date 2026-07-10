package generator

import (
	"os"
	"path/filepath"
	"strings"
)

// WriteText 以 UTF-8 + LF 换行写入文本文件，自动创建父目录。
// 对应 Python 的 write_text（newline="\n" 强制 LF，即使 Windows 也写 LF）。
func WriteText(path, content string) error {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	// 强制 LF（去掉可能存在的 CR），保证跨平台一致。
	content = strings.ReplaceAll(content, "\r\n", "\n")
	return os.WriteFile(path, []byte(content), 0o644)
}

// WriteExecutable 以 UTF-8 + LF 写入并赋予 0o755（对应 Python write_executable）。
// 不自动创建父目录（与 Python 一致，调用方需保证父目录存在）。
func WriteExecutable(path, content string) error {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		return err
	}
	_ = os.Chmod(path, 0o755) // Windows 上无 Unix 执行位概念，忽略错误
	return nil
}

// WriteBat 写入 Windows .bat 脚本：UTF-8 BOM + CRLF。
// 对应 Python 的 write_bat（chcp 65001 + BOM 才能正确显示中文）。
func WriteBat(path, content string) error {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	// 先把已有 CRLF 折叠为 LF，再统一展开为 CRLF，保证纯 CRLF。
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\n", "\r\n")
	buf := make([]byte, 0, len(content)+3)
	buf = append(buf, 0xEF, 0xBB, 0xBF) // UTF-8 BOM
	buf = append(buf, []byte(content)...)
	return os.WriteFile(path, buf, 0o644)
}
