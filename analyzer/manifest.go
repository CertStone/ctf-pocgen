// Package analyzer 负责分析 JAR 结构、解析 MANIFEST、识别 JDK 版本与项目类型。
//
// 本包是对原 Python 版 create-ctf-poc.py 中 Analyzer 层 + 新增类型检测的 Go 复刻。
// 仅依赖 Go 标准库（archive/zip、regexp、strings 等）。
package analyzer

import (
	"archive/zip"
	"io"
	"regexp"
	"strings"
)

// majorToSource 对应 Python 的 _MAJOR_TO_SOURCE。
// 注意："3"/"4" 刻意缺失，避免把 Maven 版本号（如 3.8.1）误判为 JDK 3。
var majorToSource = map[string]string{
	"1.1": "1.1", "1.2": "1.2", "1.3": "1.3", "1.4": "1.4",
	"1.5": "1.5", "1.6": "1.6", "1.7": "1.7", "1.8": "1.8",
	"5": "1.5", "6": "1.6", "7": "1.7", "8": "1.8",
	"9": "9", "10": "10", "11": "11", "12": "12", "13": "13",
	"14": "14", "15": "15", "16": "16", "17": "17", "18": "18",
	"19": "19", "20": "20", "21": "21",
}

// versionTokenRe 匹配版本号 token：数字开头，可带 0~3 个 [.或_]后跟数字的分组。
// 等价于 Python 的 r"(\d+(?:[._]\d+){0,3})"。
var versionTokenRe = regexp.MustCompile(`(\d+(?:[._]\d+){0,3})`)

// verToSource 把单个版本号 token 归一化为 maven.compiler.source 取值。
// 对应 Python 的 _ver_to_source。
func verToSource(ver string) string {
	ver = strings.ReplaceAll(ver, "_", ".")
	if strings.HasPrefix(ver, "1.") {
		parts := strings.Split(ver, ".")
		if len(parts) >= 2 {
			head := parts[0] + "." + parts[1] // "1.8"
			if v, ok := majorToSource[head]; ok {
				return v
			}
		}
		return ""
	}
	major := strings.Split(ver, ".")[0]
	return majorToSource[major] // map 零值是 ""，等价于 Python None
}

// NormalizeJDKVersion 把任意形式的 JDK 版本字符串归一化为 maven.compiler.source 取值。
// 对应 Python 的 normalize_jdk_version。
// 策略：抽取所有版本号 token，逐个尝试映射，返回首个可识别的。
func NormalizeJDKVersion(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	for _, tok := range versionTokenRe.FindAllString(raw, -1) {
		if v := verToSource(tok); v != "" {
			return v
		}
	}
	return ""
}

// createdByToolRe 匹配 Created-By 里构建工具自身的标识及其版本号，
// 用于剔除工具版本（如 "Gradle 8.5" 中的 8.5、"Maven Jar Plugin 3.2.0" 中的 3.2.0），
// 避免 Gradle 8.5 被误读为 JDK 8。
//
// 覆盖的常见 Created-By 形态（来自真实 manifest 与官方文档）：
//   - "Gradle 8.5"
//   - "Maven Jar Plugin 3.2.0"
//   - "Apache Maven 3.8.1"
//   - "Maven Archiver 3.6.0"
//   - "Ant 1.10.12"
//   - "Gradle 8.10.2"
//
// 匹配 "工具名 + 可选空格 + 版本号"，把它们整段移除，剩下的（如 "(1.8.0_292)"、"(Java 11)"）
// 才是真正的 JDK 信号。
var createdByToolRe = regexp.MustCompile(`(?i)(gradle|maven|archiver|ant|ivy|sbt)\s*\d+(?:[._]\d+)*`)

// NormalizeCreatedByJDK 专门解析 Created-By 字段中的 JDK 版本。
//
// 与通用的 NormalizeJDKVersion 的区别：先剔除构建工具自身的版本号
// （这是 Created-By 特有的陷阱），再在剩余文本中寻找 JDK 版本。
//
// 例：
//
//	"Gradle 8.5"                       -> "" （无 JDK 信号）
//	"Maven Jar Plugin 3.2.0 (1.8.0_292)" -> "1.8"
//	"Apache Maven 3.8.1 (Java 11)"     -> "11"
//	"Ant 1.10.12"                      -> ""
func NormalizeCreatedByJDK(createdBy string) string {
	if strings.TrimSpace(createdBy) == "" {
		return ""
	}
	// 先剔除工具名 + 版本号
	cleaned := createdByToolRe.ReplaceAllString(createdBy, " ")
	return NormalizeJDKVersion(cleaned)
}

// JDKDetail 记录 JDK 版本检测的来源信息（对应 Python 返回的 detail dict）。
type JDKDetail struct {
	// Build-Jdk-Spec / Build-Jdk / Created-By 的原始值（可能为空）。
	BuildJdkSpec string
	BuildJdk     string
	CreatedBy    string
	// 默认兜底标记（仅当走了默认 1.8 分支时为 "1.8"）。
	Default string
	// force-jdk 强制标记（仅当走 force 分支时非空）。
	Forced string
}

// HasValue 用于 README 渲染：判断 detail 中是否有非空来源。
func (d JDKDetail) Entries() [][2]string {
	var out [][2]string
	if d.Forced != "" {
		out = append(out, [2]string{"forced", d.Forced})
		return out
	}
	if d.BuildJdkSpec != "" {
		out = append(out, [2]string{"Build-Jdk-Spec", d.BuildJdkSpec})
	}
	if d.BuildJdk != "" {
		out = append(out, [2]string{"Build-Jdk", d.BuildJdk})
	}
	if d.CreatedBy != "" {
		out = append(out, [2]string{"Created-By", d.CreatedBy})
	}
	if d.Default != "" {
		out = append(out, [2]string{"default", d.Default})
	}
	return out
}

// DetectJDKFromManifest 根据 MANIFEST.MF 文本解析 JDK 版本。
// 对应 Python 的 detect_jdk_from_manifest。
//
// 优先级（force 优先覆盖一切）：
//  1. force_jdk（非空时直接返回，值为 normalize 结果或原始串）
//  2. Build-Jdk-Spec
//  3. Build-Jdk
//  4. Created-By 内嵌版本号
//  5. 默认 "1.8"
//
// 返回 (source版本, detail)。
func DetectJDKFromManifest(manifestText, forceJDK string) (string, JDKDetail) {
	if strings.TrimSpace(forceJDK) != "" {
		norm := NormalizeJDKVersion(forceJDK)
		if norm == "" {
			norm = strings.TrimSpace(forceJDK) // 与 Python 一致：无法归一化则用原始串
		}
		return norm, JDKDetail{Forced: forceJDK}
	}

	headers := parseManifestHeaders(manifestText)
	d := JDKDetail{
		BuildJdkSpec: headers["build-jdk-spec"],
		BuildJdk:     headers["build-jdk"],
		CreatedBy:    headers["created-by"],
	}
	if v := NormalizeJDKVersion(d.BuildJdkSpec); v != "" {
		return v, d
	}
	if v := NormalizeJDKVersion(d.BuildJdk); v != "" {
		return v, d
	}
	if v := NormalizeCreatedByJDK(d.CreatedBy); v != "" {
		return v, d
	}
	d.Default = "1.8"
	return "1.8", d
}

// parseManifestHeaders 把 MANIFEST.MF 文本解析为 key 小写的 map。
// 对应 Python detect_jdk_from_manifest 中的解析逻辑（简化：不合并续行）。
func parseManifestHeaders(manifestText string) map[string]string {
	headers := make(map[string]string)
	if manifestText == "" {
		return headers
	}
	for _, line := range strings.Split(manifestText, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, ":") {
			continue
		}
		// 只取单行 key: value（partition ":"）
		idx := strings.Index(line, ":")
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		headers[strings.ToLower(key)] = value
	}
	return headers
}

// ReadManifest 从 fat jar 读取 META-INF/MANIFEST.MF 文本，失败返回空串。
// 对应 Python 的 read_manifest。
// 策略：先精确匹配标准路径，失败则大小写不敏感回退。
func ReadManifest(jarPath string) (string, error) {
	r, err := zip.OpenReader(jarPath)
	if err != nil {
		return "", nil // 与 Python 一致：BadZipFile 时返回 ""（不抛错）
	}
	defer r.Close()

	// 精确匹配
	for _, f := range r.File {
		if f.Name == "META-INF/MANIFEST.MF" {
			return readEntryText(f)
		}
	}
	// 大小写不敏感回退
	for _, f := range r.File {
		if strings.EqualFold(f.Name, "meta-inf/manifest.mf") {
			return readEntryText(f)
		}
	}
	return "", nil
}

// readEntryText 读取一个 zip 条目的文本（UTF-8，错误用 Replacement 字符替代）。
func readEntryText(f *zip.File) (string, error) {
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}
	// 等价于 Python 的 decode("utf-8", "replace")：无效字节替换为 U+FFFD。
	return strings.ToValidUTF8(string(data), "�"), nil
}

// LanguageLevel 把 maven.compiler.source 取值映射成 IDEA 的 Language level 显示值。
// 对应 Python 的 language_level。例："1.8"->"8"，"1.7"->"7"，"11"->"11"。
func LanguageLevel(jdkVersion string) string {
	if jdkVersion == "" {
		return "8"
	}
	if strings.HasPrefix(jdkVersion, "1.") {
		parts := strings.Split(jdkVersion, ".")
		return parts[len(parts)-1]
	}
	return jdkVersion
}

// ManifestHeader 从 manifest 文本中按 key 提取首个值（大小写不敏感前缀匹配）。
// 对应 Python analyze_jar 里逐行解析 Start-Class/Main-Class 等的逻辑。
func ManifestHeader(manifestText, key string) string {
	prefix := strings.ToLower(key) + ":"
	for _, line := range strings.Split(manifestText, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), prefix) {
			// 去掉第一个 ":" 之后的内容
			idx := strings.Index(line, ":")
			return strings.TrimSpace(line[idx+1:])
		}
	}
	return ""
}
