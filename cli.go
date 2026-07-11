package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ctf-pocgen/pipeline"
)

const description = "CTF SpringBoot JAR 一键 POC 项目生成工具 —— " +
	"从 fat jar/war 生成零污染、高保真、可立即编译运行的 Maven POC 项目"

const epilog = `示例:
  ctf-pocgen challenge.jar
  ctf-pocgen challenge.jar my-poc
  ctf-pocgen challenge.jar --force-jdk 11
  ctf-pocgen challenge.jar --exclude-jars 'log4j*,slf4j*'
  ctf-pocgen                        # 无参数启动 TUI 文件浏览界面
`

// die 对应 Python 的 die：输出到 stderr 并退出 1。
func die(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "[-] "+format+"\n", a...)
	os.Exit(1)
}

func warn(format string, a ...interface{}) {
	fmt.Fprintf(os.Stdout, "[!] "+format+"\n", a...)
}

// parseExcludePatterns 把 'log4j*,slf4j*' 解析成 ['log4j*','slf4j*']。
func parseExcludePatterns(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// valueFlags 是需要带值（空格分隔）的 flag 名称集合。
// boolean flag（如 --force）不在此列。
var valueFlags = map[string]bool{
	"-force-jdk":     true,
	"--force-jdk":    true,
	"-exclude-jars":  true,
	"--exclude-jars": true,
}

// intersperseArgs 把参数重排为「所有 flag 提前、位置参数在后」的顺序，
// 让 Go flag 包也能像 Python argparse 一样允许位置参数与 flag 混排。
//
// 例：["a.jar", "poc", "--force-jdk", "17"] -> ["--force-jdk", "17", "a.jar", "poc"]
//
// 处理规则：遍历 args；
//   - 若 token 以 "-" 开头且是需要值的 flag，则它和下一个 token 一起视为 flag 部分
//   - 若 token 以 "-" 开头但不需要值（如 --force、-h），单独视为 flag
//   - 否则为位置参数
func intersperseArgs(args []string) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
			// 支持 "--flag=value" 形式（无需额外 token）
			if strings.Contains(a, "=") {
				continue
			}
			if valueFlags[a] && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
		} else {
			positional = append(positional, a)
		}
	}
	return append(flags, positional...)
}

// runCLI 处理带参数的命令行调用。
func runCLI(args []string) {
	// 重新构造 flag 集合，避免与 main 中的全局 flag 冲突
	fs := flag.NewFlagSet("ctf-pocgen", flag.ContinueOnError)
	fs.Usage = func() {
		out := fs.Output()
		fmt.Fprintf(out, "用法: %s [jar_path] [project_name] [flags]\n\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(out, "%s\n\n", description)
		fmt.Fprint(out, epilog)
		fmt.Fprintln(out, "Flags:")
		fs.PrintDefaults()
	}
	forceJDK := fs.String("force-jdk", "", "强制指定 JDK 版本（如 1.8 / 11 / 17），覆盖自动检测")
	excludeJars := fs.String("exclude-jars", "", "逗号分隔的 glob 模式，匹配的 lib jar 不引入（如 'log4j*,slf4j*'）")
	force := fs.Bool("force", false, "目标项目目录已存在时强制覆盖（先删除再生成）")
	openIDEA := fs.Bool("open-idea", false, "生成后自动用 IntelliJ IDEA 打开项目")

	// Go flag 默认在遇到首个非 flag 参数后停止解析（不支持 interspersed）。
	// 为与 Python argparse 行为一致（允许 jar 与 flag 混排），先把参数重排为
	// 「所有 flag 提前、位置参数在后」的顺序，再交给 flag 解析。
	args = intersperseArgs(args)

	if err := fs.Parse(args); err != nil {
		// flag 包已打印错误；若是 -h 则正常退出
		if err == flag.ErrHelp {
			os.Exit(0)
		}
		os.Exit(2)
	}

	rest := fs.Args()
	if len(rest) == 0 {
		// 无位置参数 → TUI
		runTUI()
		return
	}
	jarPath := rest[0]
	var projectName string
	if len(rest) > 1 {
		projectName = rest[1]
	}

	// 解析为绝对路径
	jarPath, _ = filepath.Abs(jarPath)

	// 先校验输入文件存在，再解析项目名（避免短文件名触发越界）
	if _, err := os.Stat(jarPath); err != nil {
		die("文件不存在：%s", jarPath)
	}
	projectName = pipeline.ResolveProjectName(jarPath, projectName)
	projectDir, _ := filepath.Abs(projectName)

	// 处理目录已存在
	if _, err := os.Stat(projectDir); err == nil {
		if *force {
			warn("项目目录已存在，--force 模式将删除重建：%s", projectDir)
			if err := os.RemoveAll(projectDir); err != nil {
				die("删除目录失败: %v", err)
			}
		} else {
			die("项目目录已存在：%s\n    使用 --force 强制覆盖，或换一个项目名。", projectDir)
		}
	}

	info("输入 JAR: %s", jarPath)
	info("输出项目: %s", projectDir)
	info("分析 JAR 结构 ...")

	opts := pipeline.Options{
		ForceJDK:        *forceJDK,
		ExcludePatterns: parseExcludePatterns(*excludeJars),
		OpenIDEA:        *openIDEA,
	}
	if len(opts.ExcludePatterns) > 0 {
		info("排除模式: %s", strings.Join(opts.ExcludePatterns, ", "))
	}

	result, err := pipeline.Run(jarPath, projectDir, projectName, opts, true)
	if err != nil {
		die("%v", err)
	}

	fmt.Print(pipeline.FinalSuccessMessage(result.ProjectDir))
}

func info(format string, a ...interface{}) {
	fmt.Fprintf(os.Stdout, "[*] "+format+"\n", a...)
}
