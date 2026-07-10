package generator

import (
	"strconv"
	"strings"
	"time"

	"ctf-pocgen/analyzer"
)

// READMEParams 是 GenerateReadme 的参数集合。
type READMEParams struct {
	ProjectName              string
	JarPath                  string // 用于显示的 basename
	JDKVersion               string
	JDKDetail                analyzer.JDKDetail
	LibJars                  []string
	ClassesCount             int
	ChallengeClassesFilename string // 为空时 README 显示 "(无)"
	StartClass               string
	SpringBootVersion        string
	ImplementationTitle      string
	// Timestamp 覆盖（测试用）；为零则用当前时间。
	Timestamp time.Time
}

// GenerateReadme 生成 README.md（对应 Python 的 generate_readme）。
// 模板逐字复刻，用 \x00...\x00 占位符替换避开 ${} 与反引号冲突。
func GenerateReadme(p READMEParams) string {
	// 动态字段构造
	libList := "  - （无第三方依赖）"
	if len(p.LibJars) > 0 {
		var b strings.Builder
		for _, j := range p.LibJars {
			b.WriteString("  - `")
			b.WriteString(j)
			b.WriteString("`\n")
		}
		libList = strings.TrimRight(b.String(), "\n")
	}

	// detail 来源行
	var detailLines []string
	for _, kv := range p.JDKDetail.Entries() {
		detailLines = append(detailLines, "- "+kv[0]+": `"+kv[1]+"`")
	}
	detailBlock := "- （未解析到具体字段，使用默认值）"
	if len(detailLines) > 0 {
		detailBlock = strings.Join(detailLines, "\n")
	}

	// fat jar 元信息
	var metaLines []string
	if p.StartClass != "" {
		metaLines = append(metaLines, "- Start-Class: `"+p.StartClass+"`")
	}
	if p.SpringBootVersion != "" {
		metaLines = append(metaLines, "- Spring Boot: `"+p.SpringBootVersion+"`")
	}
	if p.ImplementationTitle != "" {
		metaLines = append(metaLines, "- Implementation-Title: `"+p.ImplementationTitle+"`")
	}
	metaBlock := "- （无）"
	if len(metaLines) > 0 {
		metaBlock = strings.Join(metaLines, "\n")
	}

	ts := p.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	tsStr := ts.Format("2006-01-02 15:04:05")

	ccf := p.ChallengeClassesFilename
	if ccf == "" {
		ccf = "(无)"
	}

	jdkLv := analyzer.LanguageLevel(p.JDKVersion)

	replacer := strings.NewReplacer(
		"\x00NAME\x00", p.ProjectName,
		"\x00TS\x00", tsStr,
		"\x00JAR\x00", p.JarPath,
		"\x00JDK\x00", p.JDKVersion,
		"\x00CC_CNT\x00", strconv.Itoa(p.ClassesCount),
		"\x00LIB_CNT\x00", strconv.Itoa(len(p.LibJars)),
		"\x00META\x00", metaBlock,
		"\x00DETAIL\x00", detailBlock,
		"\x00CCF\x00", ccf,
		"\x00LIB_LIST\x00", libList,
		"\x00JDK_LV\x00", jdkLv,
	)
	return replacer.Replace(readmeTemplate)
}
