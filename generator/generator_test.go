package generator

import (
	"os"
	"strings"
	"testing"
)

func TestMakeArtifactID(t *testing.T) {
	cases := []struct{ in, want string }{
		{"commons-collections-3.2.1.jar", "commons-collections-3-2-1"},
		{"spring-web-5.2.8.RELEASE.jar", "spring-web-5-2-8-release"},
		{"jakarta.annotation-api-1.3.5.jar", "jakarta-annotation-api-1-3-5"},
		{"my.jar", "my"},
		{"....jar", "unnamed"},
		{"UPPER Case.JAR", "upper-case"},
	}
	for _, c := range cases {
		if got := MakeArtifactID(c.in); got != c.want {
			t.Errorf("MakeArtifactID(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestXMLEscape(t *testing.T) {
	if got := XMLEscape("a<b>&c"); got != "a&lt;b&gt;&amp;c" {
		t.Errorf("XMLEscape = %q", got)
	}
}

// TestGeneratePOM 验证 challenge-classes 在首位、${project.basedir} 字面保留、
// groupId 区分、JDK 与 artifactId 正确。
func TestGeneratePOM(t *testing.T) {
	pom := GeneratePOM(POMParams{
		ProjectName:              "poc-test",
		JDKVersion:               "1.8",
		LibJars:                  []string{"commons-collections-3.2.1.jar"},
		ChallengeClassesFilename: "challenge-classes.jar",
		StartClass:               "com.App",
		SpringBootVersion:        "2.3.2.RELEASE",
	})
	checks := []struct{ desc, sub string }{
		{"maven.compiler.source 1.8", "<maven.compiler.source>1.8</maven.compiler.source>"},
		{"artifactId from project name", "<artifactId>poc-test</artifactId>"},
		{"challenge groupId first", "<groupId>ctf.challenge</groupId>"},
		{"challenge systemPath literal basedir", "${project.basedir}/lib/challenge-classes.jar"},
		{"lib groupId", "<groupId>ctf.lib</groupId>"},
		{"lib artifactId normalized", "<artifactId>commons-collections-3-2-1</artifactId>"},
		{"lib systemPath keeps original filename", "${project.basedir}/lib/commons-collections-3.2.1.jar"},
		{"exec plugin mainClass", "<mainClass>ctf.poc.Poc</mainClass>"},
		{"Start-Class comment", "<!-- 题目 Start-Class: com.App -->"},
		{"Spring Boot version comment", "<!-- Spring Boot 版本: 2.3.2.RELEASE -->"},
	}
	for _, c := range checks {
		if !strings.Contains(pom, c.sub) {
			t.Errorf("%s: POM 缺少片段 %q", c.desc, c.sub)
		}
	}
	// challenge 依赖必须出现在 lib 依赖之前
	ccIdx := strings.Index(pom, "ctf.challenge")
	libIdx := strings.Index(pom, "ctf.lib")
	if ccIdx < 0 || libIdx < 0 || ccIdx > libIdx {
		t.Errorf("challenge-classes 依赖未在第三方依赖之前: ccIdx=%d libIdx=%d", ccIdx, libIdx)
	}
}

// TestGeneratePOM_NoChallengeClasses 验证 classes 为空时省略 challenge 依赖。
func TestGeneratePOM_NoChallengeClasses(t *testing.T) {
	pom := GeneratePOM(POMParams{
		ProjectName:              "poc-empty",
		JDKVersion:               "11",
		LibJars:                  []string{"foo-1.0.jar"},
		ChallengeClassesFilename: "",
	})
	if strings.Contains(pom, "ctf.challenge") {
		t.Errorf("不应生成 challenge 依赖")
	}
	if !strings.Contains(pom, "<maven.compiler.source>11</maven.compiler.source>") {
		t.Errorf("JDK 11 未生效")
	}
}

// TestWriteBat_NoBOM 回归保护：.bat 必须不含 UTF-8 BOM，且模板为纯 ASCII。
// BOM（EF BB BF）会被中文 Windows 的 cmd.exe 按 GBK 解码为「锘緻」拼到首行命令前，
// 导致 `@echo off` 变成 `锘緻echo off` 而报「不是内部或外部命令」。
// 此外 cmd.exe 按 OEM 代码页（中文系统 936/GBK）读取/解析 .bat 文件内容，
// chcp 65001 只影响控制台输出、不影响 .bat 自身解析，故模板里的 REM/echo 文本
// 必须是纯 ASCII，否则中文会被 GBK 误读导致 '锟' is not recognized 报错。
// 同时验证换行为 CRLF，java 调用含 UTF-8 编码参数（保证 POC 中文输出正确）。
func TestWriteBat_NoBOM(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/compile-run.bat"
	if err := WriteBat(path, CompileRunBAT); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// 必须不以 BOM 开头
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		t.Errorf("compile-run.bat 不应包含 UTF-8 BOM（会导致 cmd.exe 首行命令损坏）")
	}
	// 必须以 @echo off 开头
	if string(data[:len("@echo off")]) != "@echo off" {
		t.Errorf("首行应以 @echo off 开头，实际前 %d 字节: %q", len("@echo off"), string(data[:12]))
	}
	// 必须用 CRLF 换行
	if !contains(data, []byte("\r\n")) {
		t.Errorf("compile-run.bat 应使用 CRLF 换行")
	}
	// 模板必须为纯 ASCII（中文会导致 cmd.exe GBK 解析报错）
	for i, b := range data {
		if b > 0x7F {
			t.Errorf("compile-run.bat 含非 ASCII 字节 0x%02x @ offset %d（.bat 必须 ASCII-only）", b, i)
			break
		}
	}
	// java 调用应含 UTF-8 编码参数（保证 POC 中文输出正确）
	if !contains(data, []byte("-Dstdout.encoding=UTF-8")) {
		t.Errorf("compile-run.bat 的 java 调用应含 -Dstdout.encoding=UTF-8")
	}
}

func contains(hay, needle []byte) bool {
	return bytesIndex(hay, needle) >= 0
}

func bytesIndex(hay, needle []byte) int {
nloop:
	for i := 0; i+len(needle) <= len(hay); i++ {
		for j := 0; j < len(needle); j++ {
			if hay[i+j] != needle[j] {
				continue nloop
			}
		}
		return i
	}
	return -1
}
