package handlers

import (
	"fmt"
	"path/filepath"
	"strings"

	"ctf-pocgen/analyzer"
	"ctf-pocgen/extractor"
	"ctf-pocgen/generator"
)

// WARHandler 处理 WAR 包（普通 WAR 与 Spring Boot WAR）。
// 采用与 fat jar 类比的统一映射：
//   - WEB-INF/classes/ → challenge-classes.jar
//   - WEB-INF/lib/*.jar → lib/
//   - Spring Boot WAR 额外纳入 WEB-INF/lib-provided/*.jar
type WARHandler struct {
	// IncludeLibProvided 为 true 时纳入 WEB-INF/lib-provided/（Spring Boot WAR）。
	IncludeLibProvided bool
}

func (h WARHandler) Handle(jarPath, projectDir, projectName string, opts Options) error {
	names, manifest, err := analyzer.AnalyzeArchive(jarPath)
	if err != nil {
		return err
	}

	jdkVersion, jdkDetail := analyzer.DetectJDKFromManifest(manifest, opts.ForceJDK)

	if err := generator.EnsureProjectDirs(projectDir); err != nil {
		return err
	}
	libDir := filepath.Join(projectDir, "lib")

	// 1) WEB-INF/classes/ → challenge-classes.jar
	var classEntries []string
	for _, n := range names {
		if strings.HasPrefix(n, "WEB-INF/classes/") && !strings.HasSuffix(n, "/") {
			classEntries = append(classEntries, n)
		}
	}
	const ccFilename = "challenge-classes.jar"
	var challengeFilename string
	if len(classEntries) > 0 {
		if _, err := extractor.ExtractClasses(jarPath, classEntries, "WEB-INF/classes/", filepath.Join(libDir, ccFilename)); err != nil {
			return fmt.Errorf("打包 challenge-classes 失败: %w", err)
		}
		challengeFilename = ccFilename
	}

	// 2) WEB-INF/lib/*.jar（+ lib-provided 若启用）
	var libEntries []string
	libEntries = append(libEntries, analyzer.ListEntriesUnder(names, "WEB-INF/lib/", ".jar")...)
	if h.IncludeLibProvided {
		libEntries = append(libEntries, analyzer.ListEntriesUnder(names, "WEB-INF/lib-provided/", ".jar")...)
	}
	var libJarsForPOM []string
	if len(libEntries) > 0 {
		copied, _, err := extractor.ExtractLibJars(jarPath, libEntries, libDir, opts.ExcludePatterns)
		if err != nil {
			return fmt.Errorf("复制依赖 jar 失败: %w", err)
		}
		libJarsForPOM = copied
	}

	startClass := analyzer.ManifestHeader(manifest, "Start-Class")
	sbVersion := analyzer.ManifestHeader(manifest, "Spring-Boot-Version")
	implTitle := analyzer.ManifestHeader(manifest, "Implementation-Title")

	return generator.RenderProject(generator.ProjectInput{
		ProjectDir:               projectDir,
		ProjectName:              projectName,
		JarBasename:              filepath.Base(jarPath),
		JDKVersion:               jdkVersion,
		JDKDetail:                jdkDetail,
		StartClass:               startClass,
		SpringBootVersion:        sbVersion,
		ImplementationTitle:      implTitle,
		ChallengeClassesFilename: challengeFilename,
		LibJars:                  libJarsForPOM,
		ClassesCount:             len(classEntries),
	})
}
