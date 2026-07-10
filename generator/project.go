package generator

import (
	"os"
	"path/filepath"
	"time"

	"ctf-pocgen/analyzer"
)

// ProjectInput 是所有 handler 收集完数据后，交给公共渲染核心的统一输入。
// 不同归档类型（fat jar / war / plain jar）只需填好这些字段，渲染产物完全一致。
type ProjectInput struct {
	ProjectDir          string // 项目输出根目录（绝对路径）
	ProjectName         string // 项目名（影响 pom artifactId、README 标题）
	JarBasename         string // 源 jar 文件名（仅用于 README 显示）
	JDKVersion          string
	JDKDetail           analyzer.JDKDetail
	StartClass          string
	SpringBootVersion   string
	ImplementationTitle string

	// ChallengeClassesFilename: 写入 lib/ 的题目类 jar 文件名。
	// 为空字符串表示不生成（如普通 jar 把自身当作它时仍填 "challenge-classes.jar"）。
	ChallengeClassesFilename string

	// LibJars: 实际复制到 lib/ 的第三方 jar 文件名列表（原始文件名，按复制顺序）。
	LibJars []string

	// ClassesCount: 题目类的条目数（用于 README 显示；普通 jar 可填 0 或估计值）。
	ClassesCount int

	// Timestamp 覆盖（测试用）；为零则用当前时间。
	Timestamp time.Time
}

// RenderProject 执行所有类型共享的「生成产物」步骤：
// 写 pom.xml / Poc.java / README.md / compile-run.sh / compile-run.bat / .gitignore。
// 调用前 lib/ 目录下应已放好 challenge-classes.jar 与各 lib jar。
//
// 对应 Python main 流程的步骤 13~16，但抽出来供多 handler 复用。
func RenderProject(in ProjectInput) error {
	// pom.xml
	pom := GeneratePOM(POMParams{
		ProjectName:              in.ProjectName,
		JDKVersion:               in.JDKVersion,
		LibJars:                  in.LibJars,
		ChallengeClassesFilename: in.ChallengeClassesFilename,
		StartClass:               in.StartClass,
		SpringBootVersion:        in.SpringBootVersion,
	})
	if err := WriteText(filepath.Join(in.ProjectDir, "pom.xml"), pom); err != nil {
		return err
	}

	// Poc.java
	srcJava := filepath.Join(in.ProjectDir, "src", "main", "java", "ctf", "poc")
	if err := WriteText(filepath.Join(srcJava, "Poc.java"), PocJavaTemplate); err != nil {
		return err
	}

	// README.md
	readme := GenerateReadme(READMEParams{
		ProjectName:              in.ProjectName,
		JarPath:                  in.JarBasename,
		JDKVersion:               in.JDKVersion,
		JDKDetail:                in.JDKDetail,
		LibJars:                  in.LibJars,
		ClassesCount:             in.ClassesCount,
		ChallengeClassesFilename: in.ChallengeClassesFilename,
		StartClass:               in.StartClass,
		SpringBootVersion:        in.SpringBootVersion,
		ImplementationTitle:      in.ImplementationTitle,
		Timestamp:                in.Timestamp,
	})
	if err := WriteText(filepath.Join(in.ProjectDir, "README.md"), readme); err != nil {
		return err
	}

	// 脚本与 .gitignore
	if err := WriteExecutable(filepath.Join(in.ProjectDir, "compile-run.sh"), CompileRunSH); err != nil {
		return err
	}
	if err := WriteBat(filepath.Join(in.ProjectDir, "compile-run.bat"), CompileRunBAT); err != nil {
		return err
	}
	if err := WriteText(filepath.Join(in.ProjectDir, ".gitignore"), GitignoreContent); err != nil {
		return err
	}
	return nil
}

// EnsureProjectDirs 创建标准 Maven 目录结构（对应 Python 步骤 10）。
// lib/, src/main/java/ctf/poc/, src/main/resources/。
func EnsureProjectDirs(projectDir string) error {
	dirs := []string{
		filepath.Join(projectDir, "lib"),
		filepath.Join(projectDir, "src", "main", "java", "ctf", "poc"),
		filepath.Join(projectDir, "src", "main", "resources"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}
