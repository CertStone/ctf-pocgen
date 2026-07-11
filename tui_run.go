package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"ctf-pocgen/pipeline"
	"ctf-pocgen/tui"
)

// runTUI 启动交互式文件浏览界面。
// 流程：浏览目录 → 选中文件 → 确认 → (选生成位置) → 是否继续 → 批量生成。
func runTUI() {
	// 非交互终端（管道/CI）直接提示，避免 TUI 卡死
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Println("[*] 检测到非交互式终端，无法启动 TUI。")
		fmt.Println("    用法: ctf-pocgen <jar_path> [project_name] [flags]")
		fmt.Println("    或在交互式终端中无参数运行以进入 TUI。")
		return
	}

	cwd, _ := os.Getwd()
	m := tui.New(cwd)
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[-] TUI 运行失败: %v\n", err)
		os.Exit(1)
	}

	result, ok := final.(tui.Model)
	if !ok || result.Quit() || len(result.Selected()) == 0 {
		fmt.Println("[*] 未选择任何文件，退出。")
		return
	}

	// 批量生成每个选中的文件
	fmt.Printf("[*] 共 %d 个文件待生成。\n", len(result.Selected()))
	for i, sel := range result.Selected() {
		fmt.Printf("\n===== [%d/%d] %s =====\n", i+1, len(result.Selected()), filepath.Base(sel.JarPath))
		projectName := pipeline.ResolveProjectName(sel.JarPath, "")
		projectDir := filepath.Join(sel.OutputDir, projectName)

		// 目录已存在则提示并跳过（TUI 模式下不自动覆盖）
		if _, err := os.Stat(projectDir); err == nil {
			fmt.Fprintf(os.Stderr, "[!] 目录已存在，跳过: %s（删除后重试或改用 CLI --force）\n", projectDir)
			continue
		}

		_, err := pipeline.Run(sel.JarPath, projectDir, projectName,
			pipeline.Options{OpenIDEA: result.OpenIDEA()}, true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[-] 生成失败: %v\n", err)
			continue
		}
		fmt.Print(pipeline.FinalSuccessMessage(projectDir))
	}
	fmt.Println("[+] 全部处理完成。")
}
