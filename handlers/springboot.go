package handlers

import (
	"fmt"
	"path/filepath"

	"ctf-pocgen/analyzer"
	"ctf-pocgen/extractor"
	"ctf-pocgen/generator"
)

// SpringBootHandler 处理 Spring Boot 可执行 fat jar（BOOT-INF/）。
// 这是核心 handler，完全对应 Python 版的 main 流程。
type SpringBootHandler struct{}

func (h SpringBootHandler) Handle(jarPath, projectDir, projectName string, opts Options) error {
	// 1) 分析 fat jar 结构
	a, err := analyzer.AnalyzeSpringBootJar(jarPath)
	if err != nil {
		return err
	}

	// 2) 检测 JDK 版本
	jdkVersion, jdkDetail := analyzer.DetectJDKFromManifest(a.Manifest, opts.ForceJDK)

	// 3) 创建项目目录结构（lib/、src/...）
	if err := generator.EnsureProjectDirs(projectDir); err != nil {
		return err
	}
	libDir := filepath.Join(projectDir, "lib")

	// 4) 提取 challenge-classes.jar
	const ccFilename = "challenge-classes.jar"
	ccPath := filepath.Join(libDir, ccFilename)
	var challengeFilename string
	if len(a.ClassEntries) > 0 {
		n, err := extractor.ExtractClasses(jarPath, a.ClassEntries, "BOOT-INF/classes/", ccPath)
		if err != nil {
			return fmt.Errorf("打包 challenge-classes 失败: %w", err)
		}
		_ = n
		challengeFilename = ccFilename
	}

	// 5) 复制 BOOT-INF/lib/*.jar
	var libJarsForPOM []string
	if len(a.LibJars) > 0 {
		// 构造完整条目路径
		entries := make([]string, len(a.LibJars))
		for i, name := range a.LibJars {
			entries[i] = "BOOT-INF/lib/" + name
		}
		copied, skipped, err := extractor.ExtractLibJars(jarPath, entries, libDir, opts.ExcludePatterns)
		if err != nil {
			return fmt.Errorf("复制依赖 jar 失败: %w", err)
		}
		_ = skipped
		libJarsForPOM = copied
	}

	// 6) 渲染项目（pom/Poc.java/README/脚本）
	return generator.RenderProject(generator.ProjectInput{
		ProjectDir:               projectDir,
		ProjectName:              projectName,
		JarBasename:              filepath.Base(jarPath),
		JDKVersion:               jdkVersion,
		JDKDetail:                jdkDetail,
		StartClass:               a.StartClass,
		SpringBootVersion:        a.SpringBootVersion,
		ImplementationTitle:      a.ImplementationTitle,
		ChallengeClassesFilename: challengeFilename,
		LibJars:                  libJarsForPOM,
		ClassesCount:             len(a.ClassEntries),
	})
}
