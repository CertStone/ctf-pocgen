package generator

import (
	"regexp"
	"strings"
)

// nonArtifactRe 匹配 artifactId 中非法字符：非 [a-z0-9-]。
var nonArtifactRe = regexp.MustCompile(`[^a-z0-9-]+`)

// dashRe 匹配连续的 '-'，用于折叠。
var dashRe = regexp.MustCompile(`-+`)

// MakeArtifactID 把 jar 文件名转成合法的 Maven artifactId。
// 对应 Python 的 make_artifact_id：
//   - 去掉 .jar，全部小写
//   - 非 [a-z0-9-] 字符替换为 -
//   - 折叠连续 - 并去除首尾 -
//   - 结果为空返回 "unnamed"
//
// 例：commons-collections-3.2.1.jar -> commons-collections-3-2-1
//
//	spring-web-5.2.8.RELEASE.jar -> spring-web-5-2-8-release
func MakeArtifactID(jarFilename string) string {
	name := strings.ToLower(jarFilename)
	if strings.HasSuffix(name, ".jar") {
		name = name[:len(name)-len(".jar")]
	}
	name = nonArtifactRe.ReplaceAllString(name, "-")
	name = dashRe.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		return "unnamed"
	}
	return name
}

// XMLEscape 转义 XML 特殊字符（对应 Python 的 xml_escape）。
// 注意顺序：先替换 & 。
func XMLEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// POMParams 是 GeneratePOM 的参数集合。
type POMParams struct {
	ProjectName              string
	JDKVersion               string
	LibJars                  []string // 实际复制到 lib/ 的 jar 文件名（原始名）
	ChallengeClassesFilename string   // 为空则不生成 challenge-classes 依赖
	StartClass               string
	SpringBootVersion        string
}

// GeneratePOM 生成 pom.xml 内容（对应 Python 的 generate_pom）。
//
// 规则：
//   - challenge-classes 依赖（若有）放第一个，groupId=ctf.challenge
//   - 第三方 jar groupId=ctf.lib，version=1.0，scope=system
//   - systemPath 用 ${project.basedir}/lib/<原始文件名>（字面量，非占位符）
//
// 注意：用字符串拼接而非 text/template，避免 ${project.basedir} 与模板语法冲突。
func GeneratePOM(p POMParams) string {
	var deps []string

	// 1) challenge-classes（若提供文件名）
	if p.ChallengeClassesFilename != "" {
		deps = append(deps, `        <dependency>
            <groupId>ctf.challenge</groupId>
            <artifactId>challenge-classes</artifactId>
            <version>1.0</version>
            <scope>system</scope>
            <systemPath>${project.basedir}/lib/`+p.ChallengeClassesFilename+`</systemPath>
        </dependency>`)
	}

	// 2) 第三方 jar
	for _, jar := range p.LibJars {
		aid := MakeArtifactID(jar)
		deps = append(deps, `        <dependency>
            <groupId>ctf.lib</groupId>
            <artifactId>`+XMLEscape(aid)+`</artifactId>
            <version>1.0</version>
            <scope>system</scope>
            <systemPath>${project.basedir}/lib/`+XMLEscape(jar)+`</systemPath>
        </dependency>`)
	}

	depsBlock := strings.Join(deps, "\n")

	// comments 行（Start-Class / Spring-Boot-Version）。
	// 注意：每项不加前导缩进（与 Python 一致），由模板中的 {comments} 位置提供首行缩进，
	// 各项间用 "\n    " 连接（4 空格缩进）。
	var comments []string
	if p.StartClass != "" {
		comments = append(comments, "<!-- 题目 Start-Class: "+XMLEscape(p.StartClass)+" -->")
	}
	if p.SpringBootVersion != "" {
		comments = append(comments, "<!-- Spring Boot 版本: "+XMLEscape(p.SpringBootVersion)+" -->")
	}
	commentsBlock := strings.Join(comments, "\n    ")

	artifact := MakeArtifactID(p.ProjectName + ".jar")

	// 用占位符替换构造（保留 ${project.basedir} 等字面量不被误处理）
	replacer := strings.NewReplacer(
		"\x00NAME\x00", XMLEscape(p.ProjectName),
		"\x00ARTIFACT\x00", XMLEscape(artifact),
		"\x00JDK\x00", XMLEscape(p.JDKVersion),
		"\x00COMMENTS\x00", commentsBlock,
		"\x00DEPS\x00", depsBlock,
	)
	return replacer.Replace(pomTemplate)
}

// pomTemplate 是 pom.xml 骨架。用 \x00...\x00 作占位符避免与 ${project.basedir} 冲突。
const pomTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>

    <!-- 由 ctf-pocgen 自动生成 -->
    <!-- 项目: ` + "\x00NAME\x00" + ` -->
    <!-- 编译 JDK 版本: ` + "\x00JDK\x00" + ` -->
    ` + "\x00COMMENTS\x00" + `

    <groupId>ctf</groupId>
    <artifactId>` + "\x00ARTIFACT\x00" + `</artifactId>
    <version>1.0</version>
    <packaging>jar</packaging>

    <properties>
        <project.build.sourceEncoding>UTF-8</project.build.sourceEncoding>
        <maven.compiler.source>` + "\x00JDK\x00" + `</maven.compiler.source>
        <maven.compiler.target>` + "\x00JDK\x00" + `</maven.compiler.target>
    </properties>

    <build>
        <plugins>
            <!-- 让 system scope 的依赖也能进入运行/测试 classpath -->
            <plugin>
                <groupId>org.apache.maven.plugins</groupId>
                <artifactId>maven-dependency-plugin</artifactId>
                <version>3.6.1</version>
                <executions>
                    <execution>
                        <id>build-classpath</id>
                        <phase>none</phase>
                    </execution>
                </executions>
            </plugin>
            <plugin>
                <groupId>org.codehaus.mojo</groupId>
                <artifactId>exec-maven-plugin</artifactId>
                <version>3.1.0</version>
                <configuration>
                    <mainClass>ctf.poc.Poc</mainClass>
                </configuration>
            </plugin>
        </plugins>
    </build>

    <dependencies>
` + "\x00DEPS\x00" + `
    </dependencies>
</project>
`
