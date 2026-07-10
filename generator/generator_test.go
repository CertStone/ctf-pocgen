package generator

import (
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
