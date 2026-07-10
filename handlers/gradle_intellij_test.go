package handlers

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"ctf-pocgen/analyzer"
)

// 验证工具对三类构建器产物的 manifest 解析与类型检测：
// Maven / Gradle / IntelliJ Artifacts，以及 Gradle shadow/Spring Boot fat jar。

// buildSample 构造一个 zip 到 path，条目来自 entries（name->content）。
func buildSample(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	zw := zip.NewWriter(out)
	defer zw.Close()
	for name, content := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(content))
	}
}

// Maven shadow-style fat jar（jar-with-dependencies）：含 META-INF/maven。
func TestManifest_MavenJarWithDeps(t *testing.T) {
	dir := t.TempDir()
	jar := filepath.Join(dir, "app-1.0-jar-with-dependencies.jar")
	buildSample(t, jar, map[string]string{
		"META-INF/MANIFEST.MF":                             "Manifest-Version: 1.0\nBuild-Jdk-Spec: 1.8\n",
		"META-INF/maven/com.example/app/pom.properties":    "#Maven\ngroupId=com.example\nartifactId=app\nversion=1.0\n",
		"com/example/App.class":                            "X",
		"org/apache/commons/collections/Transformer.class": "Y",
	})
	names, manifest, err := analyzer.AnalyzeArchive(jar)
	if err != nil {
		t.Fatal(err)
	}
	if got := analyzer.DetectType(names, manifest); got != analyzer.TypePlainJar {
		t.Errorf("应为 TypePlainJar, got %v", got)
	}
	// Build-Jdk-Spec 应被解析
	v, _ := analyzer.DetectJDKFromManifest(manifest, "")
	if v != "1.8" {
		t.Errorf("JDK 应为 1.8, got %v", v)
	}
}

// Gradle shadow / Spring Boot Druid 风格：manifest 是 Gradle 生成的，
// Build-Jdk 字段形态与 Maven 不同（如 "1.8.0_292"），无 META-INF/maven。
func TestManifest_GradleShadow(t *testing.T) {
	dir := t.TempDir()
	jar := filepath.Join(dir, "app-1.0-all.jar") // Gradle shadow 常用 -all 后缀
	buildSample(t, jar, map[string]string{
		// Gradle 生成的 manifest，Build-Jdk 形态可能是 "1.8.0_292" 或 "17"
		"META-INF/MANIFEST.MF": "Manifest-Version: 1.0\nImplementation-Title: app\nImplementation-Version: 1.0\nBuild-Jdk: 1.8.0_292\nMain-Class: com.example.App\n",
		// Gradle 无 META-INF/maven，只有自己的类 + 解包的第三方类
		"com/example/App.class":                            "X",
		"org/apache/commons/collections/Transformer.class": "Y",
	})
	names, manifest, err := analyzer.AnalyzeArchive(jar)
	if err != nil {
		t.Fatal(err)
	}
	if got := analyzer.DetectType(names, manifest); got != analyzer.TypePlainJar {
		t.Errorf("Gradle shadow 应为 TypePlainJar, got %v", got)
	}
	v, _ := analyzer.DetectJDKFromManifest(manifest, "")
	if v != "1.8" {
		t.Errorf("Gradle Build-Jdk 1.8.0_292 应解析为 1.8, got %v", v)
	}
}

// Gradle Spring Boot fat jar：有 BOOT-INF（Gradle 也能产生标准 Spring Boot fat jar）。
func TestManifest_GradleSpringBoot(t *testing.T) {
	dir := t.TempDir()
	jar := filepath.Join(dir, "app.jar")
	buildSample(t, jar, map[string]string{
		"META-INF/MANIFEST.MF":                   "Manifest-Version: 1.0\nMain-Class: org.springframework.boot.loader.JarLauncher\nStart-Class: com.example.App\nBuild-Jdk: 17\nSpring-Boot-Version: 3.2.0\n",
		"BOOT-INF/classes/com/example/App.class": "X",
		"BOOT-INF/lib/spring-core-6.1.0.jar":     "SPRING",
	})
	names, manifest, err := analyzer.AnalyzeArchive(jar)
	if err != nil {
		t.Fatal(err)
	}
	if got := analyzer.DetectType(names, manifest); got != analyzer.TypeSpringBootJar {
		t.Errorf("Gradle Spring Boot fat jar 应为 TypeSpringBootJar, got %v", got)
	}
	v, _ := analyzer.DetectJDKFromManifest(manifest, "")
	if v != "17" {
		t.Errorf("JDK 应为 17, got %v", v)
	}
}

// IntelliJ Artifacts（Build → Build Artifacts）：手动配置的 manifest，
// 可能完全无 Build-Jdk/Build-Jdk-Spec/Created-By，或 Main-Class 指向应用类。
func TestManifest_IntelliJArtifact(t *testing.T) {
	dir := t.TempDir()
	jar := filepath.Join(dir, "myartifact.jar")
	buildSample(t, jar, map[string]string{
		// IntelliJ Artifacts 生成的 manifest 通常极简，可能无任何构建工具元数据
		"META-INF/MANIFEST.MF":     "Manifest-Version: 1.0\nMain-Class: com.example.Main\n",
		"com/example/Main.class":   "X",
		"com/example/Config.class": "Y",
		"log4j.properties":         "LOG4J",
	})
	names, manifest, err := analyzer.AnalyzeArchive(jar)
	if err != nil {
		t.Fatal(err)
	}
	if got := analyzer.DetectType(names, manifest); got != analyzer.TypePlainJar {
		t.Errorf("IntelliJ Artifacts 应为 TypePlainJar, got %v", got)
	}
	// 无 Build-Jdk 字段时应兜底 1.8
	v, _ := analyzer.DetectJDKFromManifest(manifest, "")
	if v != "1.8" {
		t.Errorf("无 Build-Jdk 应兜底 1.8, got %v", v)
	}
}

// 端到端：三类构建器的 fat jar 都能生成可用的 POC 项目（类完整进入 classpath）。
func TestEndToEnd_AllBuilders_PlainJar(t *testing.T) {
	dir := t.TempDir()
	for name, manifest := range map[string]string{
		"maven.jar":    "Manifest-Version: 1.0\nBuild-Jdk-Spec: 1.8\n",
		"gradle.jar":   "Manifest-Version: 1.0\nBuild-Jdk: 11.0.12\n",
		"intellij.jar": "Manifest-Version: 1.0\n", // 无任何构建元数据
	} {
		jar := filepath.Join(dir, name)
		buildSample(t, jar, map[string]string{
			"META-INF/MANIFEST.MF":                             manifest,
			"com/example/App.class":                            "X",
			"org/apache/commons/collections/Transformer.class": "Y",
		})
		projectDir := filepath.Join(dir, "out-"+name)
		os.MkdirAll(projectDir, 0o755)
		if err := (PlainJarHandler{}).Handle(jar, projectDir, "poc-"+name, Options{}); err != nil {
			t.Errorf("%s: Handle 失败: %v", name, err)
			continue
		}
		// 验证 challenge-classes.jar 存在且是源 jar 副本（类完整进入 classpath）
		cc := filepath.Join(projectDir, "lib", "challenge-classes.jar")
		orig, _ := os.ReadFile(jar)
		got, err := os.ReadFile(cc)
		if err != nil {
			t.Errorf("%s: cc.jar 缺失", name)
		} else if string(orig) != string(got) {
			t.Errorf("%s: cc.jar 应为源 jar 完整副本", name)
		}
		// pom 存在
		if _, err := os.Stat(filepath.Join(projectDir, "pom.xml")); err != nil {
			t.Errorf("%s: pom.xml 缺失", name)
		}
	}
}
