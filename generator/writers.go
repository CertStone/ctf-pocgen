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

// WriteBat 写入 Windows .bat 脚本：UTF-8（无 BOM）+ CRLF。
//
// 关键：不加 UTF-8 BOM。原因——cmd.exe 读取 .bat 时按当前 OEM 代码页解码，
// 在中文 Windows（代码页 936/GBK）下，BOM 字节 EF BB BF 会被解码为「锘緻」并
// 拼到首行命令前，导致 `@echo off` 变成 `锘緻echo off` 而报「不是内部或外部命令」。
// chcp 65001 只能影响其后的行，无法修复已被污染的首行，因此必须去掉 BOM。
//
// 配合脚本首行后的 `chcp 65001 > nul`，中文可正确显示（首行 @echo off 是纯 ASCII，
// 在任何代码页下都能解析）。模板中 chcp 必须出现在任何含中文的行之前。
func WriteBat(path, content string) error {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	// 先把已有 CRLF 折叠为 LF，再统一展开为 CRLF，保证纯 CRLF。
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\n", "\r\n")
	// UTF-8 无 BOM
	return os.WriteFile(path, []byte(content), 0o644)
}
