package analyzer

import "testing"

// 测试 NormalizeJDKVersion 的各种形式（移植自 Python 版 18 个用例）。
func TestNormalizeJDKVersion(t *testing.T) {
	cases := []struct {
		raw, want string
	}{
		{"1.8.0_292", "1.8"},
		{"1.8", "1.8"},
		{"1.7.0_80", "1.7"},
		{"11", "11"},
		{"11.0.12", "11"},
		{"11.0.12+7", "11"},
		{"17", "17"},
		{"17.0.3", "17"},
		{"21", "21"},
		// 多 token：Maven 自身版本在前，主版本 3 不在表内，应取首个可识别的
		{"Apache Maven 3.8.1 (Java 11)", "11"},
		// 嵌套括号里的版本
		{"(Ubuntu 11.0.12+7-post-Ubuntu-1)", "11"},
		// 无可识别 token
		{"no version here", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := NormalizeJDKVersion(c.raw); got != c.want {
			t.Errorf("NormalizeJDKVersion(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}

// 测试 DetectJDKFromManifest 的优先级与 force-jdk 覆盖。
func TestDetectJDKFromManifest(t *testing.T) {
	cases := []struct {
		name, manifest, force, want string
	}{
		{"Spec 1.8", "Build-Jdk-Spec: 1.8\n", "", "1.8"},
		{"Spec 11", "Build-Jdk-Spec: 11\n", "", "11"},
		{"Spec 17", "Build-Jdk-Spec: 17\n", "", "17"},
		{"Spec 21", "Build-Jdk-Spec: 21\n", "", "21"},
		{"BuildJdk 1.8.0_292", "Build-Jdk: 1.8.0_292\n", "", "1.8"},
		{"BuildJdk 11.0.12", "Build-Jdk: 11.0.12\n", "", "11"},
		{"CreatedBy 1.8.0_292", "Created-By: Maven Jar Plugin 3.2.0 (1.8.0_292)\n", "", "1.8"},
		{"CreatedBy Java 11", "Created-By: Apache Maven 3.8.1 (Java 11)\n", "", "11"},
		// 优先级：Spec(11) 胜过 Build-Jdk(1.8)
		{"Spec beats BuildJdk", "Build-Jdk-Spec: 11\nBuild-Jdk: 1.8.0_292\n", "", "11"},
		// 无字段 -> 默认 1.8
		{"default", "Manifest-Version: 1.0\n", "", "1.8"},
		// force 覆盖
		{"force 17 over Spec 1.8", "Build-Jdk-Spec: 1.8\n", "17", "17"},
	}
	for _, c := range cases {
		got, _ := DetectJDKFromManifest(c.manifest, c.force)
		if got != c.want {
			t.Errorf("%s: DetectJDKFromManifest = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestLanguageLevel(t *testing.T) {
	cases := []struct{ in, want string }{
		{"1.8", "8"}, {"1.7", "7"}, {"11", "11"}, {"17", "17"}, {"", "8"},
	}
	for _, c := range cases {
		if got := LanguageLevel(c.in); got != c.want {
			t.Errorf("LanguageLevel(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// 测试 ManifestHeader 的大小写不敏感前缀匹配。
func TestManifestHeader(t *testing.T) {
	m := "Manifest-Version: 1.0\nStart-Class: com.example.App\nMain-Class: org.springframework.boot.loader.JarLauncher\n"
	if got := ManifestHeader(m, "Start-Class"); got != "com.example.App" {
		t.Errorf("Start-Class = %q", got)
	}
	// 大小写不敏感
	if got := ManifestHeader(m, "start-class"); got != "com.example.App" {
		t.Errorf("start-class = %q", got)
	}
	if got := ManifestHeader(m, "Main-Class"); got != "org.springframework.boot.loader.JarLauncher" {
		t.Errorf("Main-Class = %q", got)
	}
	if got := ManifestHeader(m, "Missing"); got != "" {
		t.Errorf("Missing = %q, want empty", got)
	}
}

// 测试 DetectType 决策树（用名字列表模拟各类型结构）。
func TestDetectType(t *testing.T) {
	cases := []struct {
		name     string
		names    []string
		manifest string
		want     ArchiveType
	}{
		{
			"EAR",
			[]string{"META-INF/application.xml", "META-INF/MANIFEST.MF", "myapp.war"},
			"", TypeEar,
		},
		{
			"SpringBoot Fat JAR",
			[]string{"BOOT-INF/classes/com/App.class", "BOOT-INF/lib/spring-1.jar", "META-INF/MANIFEST.MF"},
			"Main-Class: org.springframework.boot.loader.JarLauncher\nStart-Class: com.App\n",
			TypeSpringBootJar,
		},
		{
			"Spring Boot 3.2+ loader path",
			[]string{"BOOT-INF/classes/com/App.class", "BOOT-INF/lib/spring-1.jar"},
			"Main-Class: org.springframework.boot.loader.launch.JarLauncher\n",
			TypeSpringBootJar,
		},
		{
			"Spring Boot WAR (BOOT-INF + WEB-INF)",
			[]string{"BOOT-INF/", "WEB-INF/classes/com/App.class", "WEB-INF/lib/x.jar"},
			"Main-Class: org.springframework.boot.loader.launch.WarLauncher\n",
			TypeSpringBootWar,
		},
		{
			"Spring Boot WAR (lib-provided)",
			[]string{"WEB-INF/classes/com/App.class", "WEB-INF/lib/x.jar", "WEB-INF/lib-provided/servlet.jar"},
			"Main-Class: org.springframework.boot.loader.launch.WarLauncher\n",
			TypeSpringBootWar,
		},
		{
			"Plain WAR",
			[]string{"WEB-INF/classes/com/Servlet.class", "WEB-INF/lib/jstl.jar", "WEB-INF/web.xml"},
			"Manifest-Version: 1.0\n",
			TypeWar,
		},
		{
			"Plain JAR",
			[]string{"META-INF/MANIFEST.MF", "com/example/Foo.class"},
			"Manifest-Version: 1.0\nBuild-Jdk: 1.8.0_292\n",
			TypePlainJar,
		},
	}
	for _, c := range cases {
		if got := DetectType(c.names, c.manifest); got != c.want {
			t.Errorf("%s: DetectType = %v, want %v", c.name, got, c.want)
		}
	}
}
