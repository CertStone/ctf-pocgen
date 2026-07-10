package analyzer

import (
	"archive/zip"
	"strings"
)

// Analysis 对应 Python analyze_jar 返回的结果 dict。
type Analysis struct {
	Names               []string // 全部条目名
	ClassEntries        []string // BOOT-INF/classes/ 下的全部条目（含资源，排除目录）
	LibJars             []string // BOOT-INF/lib/*.jar 文件名（仅文件名）
	Manifest            string   // MANIFEST.MF 文本
	StartClass          string
	MainClass           string
	SpringBootVersion   string
	ImplementationTitle string
}

// AnalyzeSpringBootJar 分析 fat jar 结构，返回与 Python analyze_jar 等价的结果。
// 若不是 Spring Boot fat jar（BOOT-INF/classes 与 BOOT-INF/lib 都缺失）返回错误。
func AnalyzeSpringBootJar(jarPath string) (*Analysis, error) {
	a := &Analysis{}

	r, err := zip.OpenReader(jarPath)
	if err != nil {
		return nil, &NotValidZipError{Path: jarPath, Err: err}
	}
	defer r.Close()

	for _, f := range r.File {
		a.Names = append(a.Names, f.Name)
	}

	manifest, _ := ReadManifest(jarPath)
	a.Manifest = manifest

	// 解析关键字段（逐行大小写不敏感前缀匹配，与 Python 一致）
	a.StartClass = ManifestHeader(manifest, "Start-Class")
	a.MainClass = ManifestHeader(manifest, "Main-Class")
	a.SpringBootVersion = ManifestHeader(manifest, "Spring-Boot-Version")
	a.ImplementationTitle = ManifestHeader(manifest, "Implementation-Title")

	// 是否为 fat jar：BOOT-INF/classes 或 BOOT-INF/lib 任一存在即可
	hasClasses := anyHasPrefix(a.Names, "BOOT-INF/classes/")
	hasLib := anyHasPrefix(a.Names, "BOOT-INF/lib/")
	if !hasClasses && !hasLib {
		return nil, &NotFatJarError{Path: jarPath}
	}

	// classes 条目（非目录）
	for _, n := range a.Names {
		if strings.HasPrefix(n, "BOOT-INF/classes/") && !strings.HasSuffix(n, "/") {
			a.ClassEntries = append(a.ClassEntries, n)
		}
	}
	// lib 下的 jar（仅文件名）
	for _, n := range a.Names {
		if strings.HasPrefix(n, "BOOT-INF/lib/") && strings.HasSuffix(n, ".jar") {
			a.LibJars = append(a.LibJars, n[len("BOOT-INF/lib/"):])
		}
	}

	return a, nil
}

// AnalyzeArchive 读取归档的全部条目名与 manifest（不做 fat jar 校验）。
// 供类型检测使用。
func AnalyzeArchive(jarPath string) (names []string, manifest string, err error) {
	r, oerr := zip.OpenReader(jarPath)
	if oerr != nil {
		return nil, "", &NotValidZipError{Path: jarPath, Err: oerr}
	}
	defer r.Close()
	for _, f := range r.File {
		names = append(names, f.Name)
	}
	manifest, _ = ReadManifest(jarPath)
	return names, manifest, nil
}

func anyHasPrefix(names []string, prefix string) bool {
	for _, n := range names {
		if strings.HasPrefix(n, prefix) {
			return true
		}
	}
	return false
}

// NotValidZipError 文件不是有效 ZIP/JAR。
type NotValidZipError struct {
	Path string
	Err  error
}

func (e *NotValidZipError) Error() string {
	return "文件不是有效的 ZIP/JAR：" + e.Path
}

// NotFatJarError 不是 Spring Boot fat jar。
type NotFatJarError struct {
	Path string
}

func (e *NotFatJarError) Error() string {
	return "未发现 BOOT-INF/ 目录，该文件似乎不是 Spring Boot 可执行 fat jar。\n" +
		"    期望路径：BOOT-INF/classes/ 和 BOOT-INF/lib/"
}
