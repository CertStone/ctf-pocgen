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

// 验证当前 PlainJarHandler 对 jar-with-dependencies 的实际产物。
func TestCurrentBehavior_JarWithDeps(t *testing.T) {
	dir := t.TempDir()
	jarPath := filepath.Join(dir, "mything-1.0-jar-with-dependencies.jar")
	makeFatJar(t, jarPath)

	projectDir := filepath.Join(dir, "out")
	os.MkdirAll(projectDir, 0o755)
	if err := (PlainJarHandler{}).Handle(jarPath, projectDir, "poc-mything", Options{}); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	// 当前行为：整个 jar 被复制为 challenge-classes.jar
	cc := filepath.Join(projectDir, "lib", "challenge-classes.jar")
	data, err := os.ReadFile(cc)
	if err != nil {
		t.Fatalf("challenge-classes.jar 应存在: %v", err)
	}
	// 该 jar 应是源 jar 的逐字节副本
	orig, _ := os.ReadFile(jarPath)
	if string(orig) != string(data) {
		t.Error("当前实现：challenge-classes.jar 应为源 jar 的完整副本")
	}

	// 问题点：题目类和第三方类混在一起，pom 里没有第三方依赖条目
	pom, _ := os.ReadFile(filepath.Join(projectDir, "pom.xml"))
	if strings.Contains(string(pom), "ctf.lib") {
		t.Error("当前实现不应有 ctf.lib 依赖（这正是问题所在）")
	}
	// 记录现状（仅用于说明，不是断言失败）
	zr, _ := zip.OpenReader(cc)
	mixed := 0
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "org/apache/commons/") || strings.HasPrefix(f.Name, "com/mycompany/") {
			mixed++
		}
	}
	zr.Close()
	t.Logf("当前产物：challenge-classes.jar 含 %d 个混合条目（题目类+第三方类混在一起），pom 无第三方依赖列表", mixed)
}
