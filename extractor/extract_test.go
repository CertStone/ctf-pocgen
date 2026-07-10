package extractor

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// makeSampleJar 构造一个 zip 到 path，条目来自 entries（name->content）。
func makeSampleJar(t *testing.T, path string, entries map[string]string) {
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

// TestExtractClasses_VerifyContentByteForByte 验证 ExtractClasses 输出的 jar
// 条目内容与源 jar 逐字节一致（保真度核心要求），且前缀被正确剥离。
func TestExtractClasses_VerifyContentByteForByte(t *testing.T) {
	dir := t.TempDir()
	jarPath := filepath.Join(dir, "app.jar")
	entries := map[string]string{
		"META-INF/MANIFEST.MF":                    "Manifest-Version: 1.0\n",
		"BOOT-INF/classes/com/App.class":          "CLASSBYTES_APP",
		"BOOT-INF/classes/com/Foo.class":          "CLASSBYTES_FOO",
		"BOOT-INF/classes/application.properties": "name=test",
	}
	makeSampleJar(t, jarPath, entries)

	outPath := filepath.Join(dir, "out", "challenge-classes.jar")
	n, err := ExtractClasses(jarPath,
		[]string{
			"BOOT-INF/classes/com/App.class",
			"BOOT-INF/classes/com/Foo.class",
			"BOOT-INF/classes/application.properties",
		},
		"BOOT-INF/classes/", outPath)
	if err != nil {
		t.Fatalf("ExtractClasses: %v", err)
	}
	if n != 3 {
		t.Errorf("写入条目数 = %d, want 3", n)
	}

	// 验证输出 jar 的条目内容与源逐字节一致，且前缀已剥离
	zr, err := zip.OpenReader(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	got := map[string]string{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		buf := new(bytes.Buffer)
		buf.ReadFrom(rc)
		rc.Close()
		got[f.Name] = buf.String()
	}

	want := map[string]string{
		"com/App.class":          "CLASSBYTES_APP",
		"com/Foo.class":          "CLASSBYTES_FOO",
		"application.properties": "name=test",
	}
	for name, content := range want {
		if got[name] != content {
			t.Errorf("条目 %s 内容不一致: got %q, want %q", name, got[name], content)
		}
	}
	if len(got) != len(want) {
		t.Errorf("输出条目数 = %d, want %d（不应有多余条目）", len(got), len(want))
	}
}

// TestExtractClasses_MissingEntryGracefullySkipped 验证 classesEntries 里
// 不存在的条目被安全跳过（不 panic、不报错）。
func TestExtractClasses_MissingEntryGracefullySkipped(t *testing.T) {
	dir := t.TempDir()
	jarPath := filepath.Join(dir, "app.jar")
	makeSampleJar(t, jarPath, map[string]string{
		"BOOT-INF/classes/com/App.class": "X",
	})
	outPath := filepath.Join(dir, "out", "cc.jar")
	n, err := ExtractClasses(jarPath,
		[]string{"BOOT-INF/classes/com/App.class", "BOOT-INF/classes/NOT_EXIST.class"},
		"BOOT-INF/classes/", outPath)
	if err != nil {
		t.Fatalf("缺失条目应被跳过而非报错: %v", err)
	}
	if n != 1 {
		t.Errorf("应只写入存在的 1 个条目, got %d", n)
	}
}

// TestExtractLibJars_ExcludePatterns 验证 exclude glob 生效。
func TestExtractLibJars_ExcludePatterns(t *testing.T) {
	dir := t.TempDir()
	jarPath := filepath.Join(dir, "app.jar")
	makeSampleJar(t, jarPath, map[string]string{
		"BOOT-INF/lib/log4j-api-2.13.jar": "LOG4J",
		"BOOT-INF/lib/keep-1.0.jar":       "KEEP",
		"BOOT-INF/lib/slf4j-api-1.7.jar":  "SLF4J",
	})
	libDir := filepath.Join(dir, "lib")
	copied, skipped, err := ExtractLibJars(jarPath,
		[]string{"BOOT-INF/lib/log4j-api-2.13.jar", "BOOT-INF/lib/keep-1.0.jar", "BOOT-INF/lib/slf4j-api-1.7.jar"},
		libDir, []string{"log4j*", "slf4j*"})
	if err != nil {
		t.Fatalf("ExtractLibJars: %v", err)
	}
	if len(copied) != 1 || copied[0] != "keep-1.0.jar" {
		t.Errorf("应只复制 keep-1.0.jar, got %v", copied)
	}
	if len(skipped) != 2 {
		t.Errorf("应跳过 2 个（log4j、slf4j）, got %v", skipped)
	}
	// 验证文件确实落盘且内容正确
	data, err := os.ReadFile(filepath.Join(libDir, "keep-1.0.jar"))
	if err != nil {
		t.Errorf("keep-1.0.jar 应落盘: %v", err)
	}
	if string(data) != "KEEP" {
		t.Errorf("keep-1.0.jar 内容错误: got %q", string(data))
	}
}

// TestCopyJarAsFile_Verbatim 验证整个 jar 被逐字节复制（普通 jar 场景）。
func TestCopyJarAsFile_Verbatim(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "lib.jar")
	makeSampleJar(t, src, map[string]string{"com/Foo.class": "X"})
	dst := filepath.Join(dir, "sub", "challenge-classes.jar")
	if err := CopyJarAsFile(src, dst); err != nil {
		t.Fatalf("CopyJarAsFile: %v", err)
	}
	a, _ := os.ReadFile(src)
	b, _ := os.ReadFile(dst)
	if !bytes.Equal(a, b) {
		t.Error("复制结果应与源逐字节一致")
	}
}
