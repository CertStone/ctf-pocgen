package analyzer

import "testing"

// 用查证到的真实 manifest 格式（来自 Spring Boot issue #32829 / #34059、
// Gradle 官方文档、JetBrains 文档以及补充资料）核实 JDK 检测行为。
// 不再用脑补格式。
func TestJDKDetection_RealWorldBuilders(t *testing.T) {
	cases := []struct {
		name     string
		manifest string
		want     string // 期望的 source 版本，或 "1.8" 表示兜底
	}{
		{
			// Maven jar-plugin 默认（含 Build-Jdk-Spec + Created-By）
			name:     "Maven 标准 fat jar",
			manifest: "Manifest-Version: 1.0\nBuild-Jdk-Spec: 1.8\nCreated-By: Maven Jar Plugin 3.2.0\n",
			want:     "1.8",
		},
		{
			// Gradle 默认 jar 任务：据 #32829，默认不写 Build-Jdk / Build-Jdk-Spec，
			// Created-By 格式是 "Gradle X.Y"，不含 JDK 版本号 → 应回落到兜底。
			name:     "Gradle 默认 jar（无 Build-Jdk，Created-By 无 JDK 号）",
			manifest: "Manifest-Version: 1.0\nCreated-By: Gradle 8.5\n",
			want:     "1.8", // 兜底（关键：不再误读 Gradle 8.5 为 1.8）
		},
		{
			// Gradle 用户若在 build.gradle 手动加了 Build-Jdk
			name:     "Gradle 用户手动加 Build-Jdk 17",
			manifest: "Manifest-Version: 1.0\nCreated-By: Gradle 8.5\nBuild-Jdk: 17.0.1\n",
			want:     "17",
		},
		{
			// 补充资料里 Gradle Build-Jdk 带厂商信息的形态
			name:     "Gradle Build-Jdk 带厂商",
			manifest: "Manifest-Version: 1.0\nBuild-Jdk: 17.0.12+7 (Eclipse Adoptium 17.0.12+7)\n",
			want:     "17",
		},
		{
			// IntelliJ Artifacts：极简，无任何构建元数据
			name:     "IntelliJ Artifacts（仅 Main-Class）",
			manifest: "Manifest-Version: 1.0\nMain-Class: com.example.Main\n",
			want:     "1.8", // 兜底
		},
		{
			// Spring Boot Gradle bootJar：Start-Class 有，Build-Jdk-Spec 可能有
			name:     "Spring Boot Gradle bootJar",
			manifest: "Manifest-Version: 1.0\nMain-Class: org.springframework.boot.loader.JarLauncher\nStart-Class: com.example.App\nBuild-Jdk-Spec: 17\n",
			want:     "17",
		},
		{
			// Maven Created-By 带 (Java N) 的形态（Created-By 能正确识别）
			name:     "Maven Created-By 带 (Java 11)",
			manifest: "Manifest-Version: 1.0\nCreated-By: Apache Maven 3.8.1 (Java 11)\n",
			want:     "11",
		},
	}
	for _, c := range cases {
		got, _ := DetectJDKFromManifest(c.manifest, "")
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// 修复回归：Gradle 版本号不被误读为 JDK。
func TestNormalizeCreatedByJDK_NoMisread(t *testing.T) {
	cases := []struct {
		in   string
		want string // 期望版本，"" 表示无信号
	}{
		{"Gradle 8.5", ""},                            // 关键：不再误读为 1.8
		{"Gradle 9.0", ""},                            // 假想未来版本，也不应误判
		{"Gradle 17.0", ""},                           // 即使版本号碰巧是 JDK 主版本，也不应识别
		{"Maven Jar Plugin 3.2.0", ""},                // Maven 工具版本也不应误读
		{"Maven Jar Plugin 3.2.0 (1.8.0_292)", "1.8"}, // 但括号里的 JDK 信号要保留
		{"Apache Maven 3.8.1 (Java 11)", "11"},        // (Java 11) 要识别
		{"Ant 1.10.12", ""},                           // Ant 版本不误读
		{"", ""},
	}
	for _, c := range cases {
		got := NormalizeCreatedByJDK(c.in)
		if got != c.want {
			t.Errorf("NormalizeCreatedByJDK(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// 确认通用 NormalizeJDKVersion 行为不变（用于 Build-Jdk-Spec / Build-Jdk）。
func TestNormalizeJDKVersion_Unchanged(t *testing.T) {
	if got := NormalizeJDKVersion("1.8.0_292"); got != "1.8" {
		t.Errorf("Build-Jdk 1.8.0_292 应解析为 1.8, got %q", got)
	}
	if got := NormalizeJDKVersion("17.0.12+7"); got != "17" {
		t.Errorf("应解析为 17, got %q", got)
	}
}
