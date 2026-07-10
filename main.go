// Command ctf-pocgen 是 CTF SpringBoot JAR 一键 POC 项目生成工具的 Go 实现。
//
// 用法：
//
//	ctf-pocgen <springboot-fatjar.jar> [project-name] [flags]
//	ctf-pocgen                              # 无参数启动 TUI 文件浏览界面
//
// 完全不污染本地 Maven 仓库（system scope + 项目内 lib/），
// 字节码与原 jar 一致，自动检测 JDK 版本与归档类型。
package main

import "os"

func main() {
	// 把 os.Args[1:] 交给 CLI 处理；无参数时由 runCLI 内部转入 TUI。
	runCLI(os.Args[1:])
}
