package analyzer

import "strings"

// ArchiveType 表示检测到的 Java 归档类型。
type ArchiveType int

const (
	TypeUnknown       ArchiveType = iota // 无法识别
	TypeSpringBootJar                    // Spring Boot 可执行 fat jar（BOOT-INF）
	TypeSpringBootWar                    // Spring Boot 可执行 WAR（WEB-INF + Boot loader）
	TypeWar                              // 普通 WAR（WEB-INF/classes 或 WEB-INF/lib）
	TypePlainJar                         // 普通 Maven JAR / 库 jar（无 BOOT-INF/WEB-INF）
	TypeEar                              // EAR（META-INF/application.xml）
)

func (t ArchiveType) String() string {
	switch t {
	case TypeSpringBootJar:
		return "Spring Boot Fat JAR"
	case TypeSpringBootWar:
		return "Spring Boot WAR"
	case TypeWar:
		return "WAR"
	case TypePlainJar:
		return "Plain JAR"
	case TypeEar:
		return "EAR"
	default:
		return "Unknown"
	}
}

// DetectType 按 zip 条目结构 + manifest 识别归档类型。
//
// 决策树（优先级从高到低，基于研究结论）：
//  1. META-INF/application.xml           -> EAR
//  2. BOOT-INF/ 且 WEB-INF/              -> Spring Boot WAR
//  3. BOOT-INF/classes 或 BOOT-INF/lib   -> Spring Boot Fat JAR
//  4. WEB-INF/classes 或 WEB-INF/lib     -> 普通 WAR
//  5. 否则                                -> 普通 JAR
//
// Spring Boot Main-Class 用前缀匹配 org.springframework.boot.loader，
// 覆盖 3.2+ 的 .loader.launch 迁移。
func DetectType(names []string, manifest string) ArchiveType {
	hasAppXml := containsName(names, "META-INF/application.xml")
	hasBootInf := anyHasPrefix(names, "BOOT-INF/")
	hasWebInf := anyHasPrefix(names, "WEB-INF/")
	hasBootClasses := anyHasPrefix(names, "BOOT-INF/classes/")
	hasBootLib := anyHasPrefix(names, "BOOT-INF/lib/")
	hasWebClasses := anyHasPrefix(names, "WEB-INF/classes/")
	hasWebLib := anyHasPrefix(names, "WEB-INF/lib/")
	hasLibProvided := anyHasPrefix(names, "WEB-INF/lib-provided/")

	// 1. EAR
	if hasAppXml {
		return TypeEar
	}
	// 2. Spring Boot WAR（既有 BOOT-INF 又有 WEB-INF；或 WEB-INF + Spring Boot loader）
	if hasBootInf && hasWebInf {
		return TypeSpringBootWar
	}
	if hasWebInf && isSpringBootLoader(manifest) {
		return TypeSpringBootWar
	}
	// 额外强信号：WEB-INF/lib-provided 是 Spring Boot WAR 专有
	if hasLibProvided && hasWebInf {
		return TypeSpringBootWar
	}
	// 3. Spring Boot Fat JAR
	if hasBootClasses || hasBootLib {
		return TypeSpringBootJar
	}
	// 4. 普通 WAR
	if hasWebClasses || hasWebLib {
		return TypeWar
	}
	// 5. 普通 JAR（默认）
	return TypePlainJar
}

// isSpringBootLoader 判断 manifest 的 Main-Class 是否指向 Spring Boot loader。
// 前缀匹配 org.springframework.boot.loader，覆盖 2.x/3.0-3.1(.loader) 与 3.2+(.loader.launch)。
func isSpringBootLoader(manifest string) bool {
	mainClass := ManifestHeader(manifest, "Main-Class")
	return strings.HasPrefix(mainClass, "org.springframework.boot.loader")
}

// containsName 判断 names 中是否存在精确等于 name 的条目（大小写敏感）。
func containsName(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

// ListEntriesUnder 返回 names 中以指定前缀开头、以 suffix 结尾的条目，
// 去掉 prefix 后返回（用于列出 WEB-INF/lib/*.jar 等）。
func ListEntriesUnder(names []string, prefix, suffix string) []string {
	var out []string
	for _, n := range names {
		if strings.HasPrefix(n, prefix) && strings.HasSuffix(n, suffix) {
			out = append(out, n)
		}
	}
	return out
}
