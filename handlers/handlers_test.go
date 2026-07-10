package handlers_test

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"ctf-pocgen/analyzer"
	"ctf-pocgen/handlers"
)

// makeZip 辅助：创建一个包含给定条目的 zip 文件到临时路径。
// entries: map[entryName]content
func makeZip(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	zw := zip.NewWriter(out)
	for name, content := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

// plainManifest 构造一个简单的 MANIFEST。
func plainManifest(jdkSpec string) string {
	return "Manifest-Version: 1.0\nBuild-Jdk-Spec: " + jdkSpec + "\n"
}

// TestSpringBootHandler_FatJar 端到端验证 Spring Boot fat jar 生成。
func TestSpringBootHandler_FatJar(t *testing.T) {
	dir := t.TempDir()
	jarPath := filepath.Join(dir, "app.jar")
	// 构造一个最小的 fat jar：1 个 class + 1 个 lib jar
	entries := map[string]string{
		"META-INF/MANIFEST.MF":                    plainManifest("1.8") + "Start-Class: com.App\nMain-Class: org.springframework.boot.loader.JarLauncher\n",
		"BOOT-INF/classes/com/App.class":          "FAKECLASSBYTES",
		"BOOT-INF/classes/application.properties": "name=app",
		"BOOT-INF/lib/spring-core-5.2.8.jar":      "FAKELIBJAR",
	}
	makeZip(t, jarPath, entries)

	projectDir := filepath.Join(dir, "out")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := (handlers.SpringBootHandler{}).Handle(jarPath, projectDir, "poc-app", handlers.Options{}); err != nil {
		t.Fatalf("SpringBootHandler.Handle: %v", err)
	}

	// 校验产物存在
	mustExist(t, filepath.Join(projectDir, "pom.xml"))
	mustExist(t, filepath.Join(projectDir, "lib", "challenge-classes.jar"))
	mustExist(t, filepath.Join(projectDir, "lib", "spring-core-5.2.8.jar"))
	mustExist(t, filepath.Join(projectDir, "src", "main", "java", "ctf", "poc", "Poc.java"))
	mustExist(t, filepath.Join(projectDir, "compile-run.sh"))
	mustExist(t, filepath.Join(projectDir, "compile-run.bat"))

	// 校验 challenge-classes.jar 含两个条目（class + properties）
	cc := filepath.Join(projectDir, "lib", "challenge-classes.jar")
	zr, err := zip.OpenReader(cc)
	if err != nil {
		t.Fatal(err)
	}
	wantEntries := map[string]bool{"com/App.class": false, "application.properties": false}
	for _, f := range zr.File {
		if _, ok := wantEntries[f.Name]; ok {
			wantEntries[f.Name] = true
		}
	}
	zr.Close()
	for name, found := range wantEntries {
		if !found {
			t.Errorf("challenge-classes.jar 缺少条目 %s", name)
		}
	}
}

// TestWARHandler 端到端验证普通 WAR 与 Spring Boot WAR 生成。
func TestWARHandler(t *testing.T) {
	dir := t.TempDir()
	jarPath := filepath.Join(dir, "app.war")
	entries := map[string]string{
		"META-INF/MANIFEST.MF":              plainManifest("11"),
		"WEB-INF/classes/com/Servlet.class": "FAKECLASS",
		"WEB-INF/lib/jstl-1.2.jar":          "FAKELIB",
		"WEB-INF/lib-provided/servlet.jar":  "PROVIDEDLIB",
	}
	makeZip(t, jarPath, entries)

	// 1) 普通 WAR（不纳入 lib-provided）
	projectDir := filepath.Join(dir, "war-out")
	os.MkdirAll(projectDir, 0o755)
	if err := (handlers.WARHandler{IncludeLibProvided: false}).Handle(jarPath, projectDir, "poc-war", handlers.Options{}); err != nil {
		t.Fatalf("WARHandler.Handle: %v", err)
	}
	mustExist(t, filepath.Join(projectDir, "lib", "challenge-classes.jar"))
	mustExist(t, filepath.Join(projectDir, "lib", "jstl-1.2.jar"))
	// lib-provided 不应被纳入
	if _, err := os.Stat(filepath.Join(projectDir, "lib", "servlet.jar")); err == nil {
		t.Error("普通 WAR 不应纳入 lib-provided")
	}

	// 2) Spring Boot WAR（纳入 lib-provided）
	projectDir2 := filepath.Join(dir, "sbwar-out")
	os.MkdirAll(projectDir2, 0o755)
	if err := (handlers.WARHandler{IncludeLibProvided: true}).Handle(jarPath, projectDir2, "poc-sbwar", handlers.Options{}); err != nil {
		t.Fatalf("WARHandler.Handle (SB): %v", err)
	}
	mustExist(t, filepath.Join(projectDir2, "lib", "servlet.jar")) // lib-provided 纳入
}

// TestPlainJarHandler 端到端验证普通 jar：自身作为 challenge-classes。
func TestPlainJarHandler(t *testing.T) {
	dir := t.TempDir()
	jarPath := filepath.Join(dir, "lib.jar")
	entries := map[string]string{
		"META-INF/MANIFEST.MF":  plainManifest("1.8"),
		"com/example/Foo.class": "FAKECLASS",
	}
	makeZip(t, jarPath, entries)

	projectDir := filepath.Join(dir, "out")
	os.MkdirAll(projectDir, 0o755)
	if err := (handlers.PlainJarHandler{}).Handle(jarPath, projectDir, "poc-lib", handlers.Options{}); err != nil {
		t.Fatalf("PlainJarHandler.Handle: %v", err)
	}
	// 普通 jar 自身被复制为 challenge-classes.jar
	cc := filepath.Join(projectDir, "lib", "challenge-classes.jar")
	mustExist(t, cc)
	// 该 cc.jar 应是源 jar 的逐字节副本（因为整个文件直接复制）
	orig, _ := os.ReadFile(jarPath)
	got, _ := os.ReadFile(cc)
	if string(orig) != string(got) {
		t.Error("普通 jar 的 challenge-classes.jar 应与源 jar 完全一致")
	}
	// pom 应只有 ctf.challenge 一个依赖（无第三方 lib）
	pom, _ := os.ReadFile(filepath.Join(projectDir, "pom.xml"))
	if !contains(string(pom), "ctf.challenge") {
		t.Error("pom 应含 ctf.challenge 依赖")
	}
	if contains(string(pom), "ctf.lib") {
		t.Error("普通 jar 的 pom 不应有 ctf.lib 依赖")
	}
}

// TestExcludePatterns 验证 --exclude-jars 在 handler 层生效。
func TestExcludePatterns(t *testing.T) {
	dir := t.TempDir()
	jarPath := filepath.Join(dir, "app.jar")
	entries := map[string]string{
		"META-INF/MANIFEST.MF":            plainManifest("1.8"),
		"BOOT-INF/classes/com/App.class":  "X",
		"BOOT-INF/lib/log4j-api-2.13.jar": "LOG4J",
		"BOOT-INF/lib/keep-1.0.jar":       "KEEP",
	}
	makeZip(t, jarPath, entries)

	projectDir := filepath.Join(dir, "out")
	os.MkdirAll(projectDir, 0o755)
	if err := (handlers.SpringBootHandler{}).Handle(jarPath, projectDir, "poc", handlers.Options{
		ExcludePatterns: []string{"log4j*"},
	}); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	mustExist(t, filepath.Join(projectDir, "lib", "keep-1.0.jar"))
	if _, err := os.Stat(filepath.Join(projectDir, "lib", "log4j-api-2.13.jar")); err == nil {
		t.Error("log4j* 应被排除")
	}
}

func mustExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("期望文件存在但缺失: %s", path)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// 确保被引用以防止编译器移除（analyzer 包用于 detect 联动测试）。
var _ = analyzer.TypePlainJar
