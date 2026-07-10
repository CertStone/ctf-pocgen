// Package pipeline 编排「检测类型 → 路由到对应 handler → 生成项目」的完整流程。
//
// 它是 CLI 与 TUI 共用的统一入口，保证两种调用方式产物完全一致。
package pipeline

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"ctf-pocgen/analyzer"
	"ctf-pocgen/handlers"
)

// Options 对应 CLI flags（与 handlers.Options 字段一致，但独立以避免循环依赖）。
type Options struct {
	ForceJDK        string
	ExcludePatterns []string
}

// Result 描述一次生成的结果，供调用方（CLI/TUI）做后续提示。
type Result struct {
	ProjectDir string
	Type       analyzer.ArchiveType
	TypeName   string
}

// logger 封装 stdout/stderr 的 [*]/[+]/[!]/[-] 前缀输出，对应 Python 消息约定。
type logger struct {
	out, errOut io.Writer
}

func newLogger() *logger {
	return &logger{out: os.Stdout, errOut: os.Stderr}
}

func (l *logger) info(format string, a ...interface{}) { fmt.Fprintf(l.out, "[*] "+format+"\n", a...) }
func (l *logger) ok(format string, a ...interface{})   { fmt.Fprintf(l.out, "[+] "+format+"\n", a...) }
func (l *logger) warn(format string, a ...interface{}) { fmt.Fprintf(l.out, "[!] "+format+"\n", a...) }

// Run 执行完整生成流程。
//
// jarPath: 源 jar 绝对路径
// projectDir: 输出项目目录绝对路径（已由调用方确定）
// projectName: 项目名
// opts: 选项
// log: 是否输出进度信息（CLI=true；TUI 批量可关闭或自行收集）
func Run(jarPath, projectDir, projectName string, opts Options, log bool) (*Result, error) {
	l := newLogger()
	if !log {
		l.out = io.Discard
	}

	// 1) 校验输入
	if _, err := os.Stat(jarPath); err != nil {
		return nil, fmt.Errorf("文件不存在：%s", jarPath)
	}

	// 2) 读取归档条目并检测类型
	names, manifest, err := analyzer.AnalyzeArchive(jarPath)
	if err != nil {
		return nil, fmt.Errorf("文件不是有效的 ZIP/JAR：%s", jarPath)
	}
	t := analyzer.DetectType(names, manifest)
	l.info("检测到类型：%s", t)

	// 3) 路由到对应 handler
	hOpts := handlers.Options{ForceJDK: opts.ForceJDK, ExcludePatterns: opts.ExcludePatterns}
	var h handlers.Handler
	switch t {
	case analyzer.TypeSpringBootJar:
		h = handlers.SpringBootHandler{}
	case analyzer.TypeSpringBootWar:
		h = handlers.WARHandler{IncludeLibProvided: true}
	case analyzer.TypeWar:
		h = handlers.WARHandler{IncludeLibProvided: false}
	case analyzer.TypePlainJar:
		h = handlers.PlainJarHandler{}
	case analyzer.TypeEar:
		return nil, fmt.Errorf(
			"检测到 EAR（企业应用归档）。EAR 是容器格式，内含若干 war/jar 模块。\n" +
				"    请先解出其中的 .war 或 .jar 后，对单个模块重新运行本工具。")
	default:
		// 兜底按普通 jar 处理
		h = handlers.PlainJarHandler{}
		l.warn("无法明确识别类型，按普通 JAR 处理")
	}

	// 4) 创建项目根目录并执行 handler
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建项目目录失败: %w", err)
	}
	if err := h.Handle(jarPath, projectDir, projectName, hOpts); err != nil {
		return nil, err
	}

	return &Result{ProjectDir: projectDir, Type: t, TypeName: t.String()}, nil
}

// ResolveProjectName 按 Python 规则解析默认项目名：poc-{jar名去.jar}。
func ResolveProjectName(jarPath, projectName string) string {
	if projectName != "" {
		return projectName
	}
	base := filepath.Base(jarPath)
	if strings.EqualFold(base[len(base)-4:], ".jar") {
		base = base[:len(base)-4]
	} else if strings.HasSuffix(strings.ToLower(base), ".war") {
		base = base[:len(base)-4]
	}
	return "poc-" + base
}

// FinalSuccessMessage 返回 Python 风格的最终成功提示块（对应 main 末尾的 print 块）。
func FinalSuccessMessage(projectDir string) string {
	return fmt.Sprintf("\n[+] 项目生成完成！\n\n"+
		"    下一步：\n"+
		"      cd %s\n"+
		"      mvn clean compile\n"+
		"      mvn exec:java        # 运行 ctf.poc.Poc\n"+
		"      # 或: bash compile-run.sh\n\n"+
		"    编写利用链：编辑 src/main/java/ctf/poc/Poc.java 的 getGadget()\n", projectDir)
}
