package handlers

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeFatJar 构造一个 jar-with-dependencies 风格的样本：
// 主项目类（com/mycompany/）+ 解包进来的第三方类（org/apache/commons/collections/）。
func makeFatJar(t *testing.T, path string) {
	t.Helper()
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	zw := zip.NewWriter(out)
	entries := map[string]string{
		"META-INF/MANIFEST.MF": "Manifest-Version: 1.0\nBuild-Jdk-Spec: 1.8\nImplementation-Title: mything\nMain-Class: com.mycompany.Main\n",
		// 主项目自身类
		"com/mycompany/Main.class":           "MAIN_CLASS",
		"com/mycompany/controller/Api.class": "API_CLASS",
		// 解包进来的第三方类（来自 commons-collections）
		"org/apache/commons/collections/Transformer.class":                 "CC_TRANSFORMER",
		"org/apache/commons/collections/functors/InvokerTransformer.class": "CC_INVOKER",
		"org/apache/commons/collections/map/LazyMap.class":                 "CC_LAZYMAP",
		// 第三方资源
		"META-INF/services/javax.script.ScriptEngineFactory": "FAKE_SERVICE",
	}
	for name, content := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(content))
	}
	zw.Close()
}

// 验证 PlainJarHandler 对 jar-with-dependencies 的产物：
// jar-with-dependencies（Maven assembly / Gradle shadow / IntelliJ Artifacts）
// 把依赖解包合并进同一 jar，故整个 jar 作为 challenge-classes 引入，
// 题目类与第三方类全部在 classpath 上可用（这是设计行为，非缺陷）。
func TestPlainJarHandler_JarWithDeps(t *testing.T) {
	dir := t.TempDir()
	jarPath := filepath.Join(dir, "mything-1.0-jar-with-dependencies.jar")
	makeFatJar(t, jarPath)

	projectDir := filepath.Join(dir, "out")
	os.MkdirAll(projectDir, 0o755)
	if err := (PlainJarHandler{}).Handle(jarPath, projectDir, "poc-mything", Options{}); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	// 整个 jar 被复制为 challenge-classes.jar（逐字节一致）
	cc := filepath.Join(projectDir, "lib", "challenge-classes.jar")
	data, err := os.ReadFile(cc)
	if err != nil {
		t.Fatalf("challenge-classes.jar 应存在: %v", err)
	}
	orig, _ := os.ReadFile(jarPath)
	if string(orig) != string(data) {
		t.Error("challenge-classes.jar 应为源 jar 的完整副本")
	}

	// pom 里只有 ctf.challenge 依赖（无独立 ctf.lib，因为依赖已合并进同一 jar）
	pom, _ := os.ReadFile(filepath.Join(projectDir, "pom.xml"))
	if !strings.Contains(string(pom), "ctf.challenge") {
		t.Error("pom 应含 ctf.challenge 依赖")
	}
	if strings.Contains(string(pom), "ctf.lib") {
		t.Error("jar-with-dependencies 的 pom 不应有独立 ctf.lib 依赖")
	}

	// challenge-classes.jar 应同时包含题目类与第三方类（都在 classpath 上）
	zr, err := zip.OpenReader(cc)
	if err != nil {
		t.Fatalf("打开 cc.jar: %v", err)
	}
	hasProject := false
	hasThirdParty := false
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "com/mycompany/") {
			hasProject = true
		}
		if strings.HasPrefix(f.Name, "org/apache/commons/") {
			hasThirdParty = true
		}
	}
	zr.Close()
	if !hasProject {
		t.Error("challenge-classes.jar 应包含题目类 (com/mycompany/)")
	}
	if !hasThirdParty {
		t.Error("challenge-classes.jar 应包含合并进来的第三方类 (org/apache/commons/)")
	}
}
