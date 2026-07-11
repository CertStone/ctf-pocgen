package handlers

import (
	"fmt"
	"path/filepath"

	"ctf-pocgen/analyzer"
	"ctf-pocgen/extractor"
	"ctf-pocgen/generator"
)

// PlainJarHandler 处理普通 Maven JAR（无 BOOT-INF/WEB-INF 的库 jar）。
// 该 jar 自身被复制进 lib/challenge-classes.jar 并作为首个 system 依赖。
// 普通 jar 不含第三方依赖，提示用户从中央仓库按需补充。
type PlainJarHandler struct{}

func (h PlainJarHandler) Handle(jarPath, projectDir, projectName string, opts Options) error {
	_, manifest, err := analyzer.AnalyzeArchive(jarPath)
	if err != nil {
		return err
	}

	jdkVersion, jdkDetail := analyzer.DetectJDKFromManifest(manifest, opts.ForceJDK)

	if err := generator.EnsureProjectDirs(projectDir); err != nil {
		return err
	}
	libDir := filepath.Join(projectDir, "lib")

	// 普通 jar 自身作为 challenge-classes 引入
	const ccFilename = "challenge-classes.jar"
	if err := extractor.CopyJarAsFile(jarPath, filepath.Join(libDir, ccFilename)); err != nil {
		return fmt.Errorf("复制 jar 作为 challenge-classes 失败: %w", err)
	}

	// 无第三方依赖（libJars 为空）；用户可后续从中央仓库补充。
	implTitle := analyzer.ManifestHeader(manifest, "Implementation-Title")

	return generator.RenderProject(generator.ProjectInput{
		ProjectDir:               projectDir,
		ProjectName:              projectName,
		JarBasename:              filepath.Base(jarPath),
		JDKVersion:               jdkVersion,
		JDKDetail:                jdkDetail,
		ImplementationTitle:      implTitle,
		ChallengeClassesFilename: ccFilename,
		LibJars:                  nil,
		ClassesCount:             0,
	})
}
