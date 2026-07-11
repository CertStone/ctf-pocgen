// Package handlers 实现各 Java 归档类型到 POC 项目的转换。
//
// 每个 Handler 完成各自归档类型特有的「类与依赖提取」逻辑，然后调用
// generator.RenderProject 生成统一的产物结构。
package handlers

// Options 是所有 handler 共享的选项（对应 CLI flags）。
type Options struct {
	ForceJDK        string   // --force-jdk
	ExcludePatterns []string // --exclude-jars 解析后
	OpenIDEA        bool     // --open-idea：生成后自动打开 IDEA
}

// Handler 处理一种归档类型。
type Handler interface {
	// Handle 执行生成。jarPath 为源 jar 绝对路径，projectDir 为输出项目目录（已创建）。
	Handle(jarPath, projectDir, projectName string, opts Options) error
}
