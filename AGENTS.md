# AGENTS.md — ctf-pocgen 架构与开发指南

本文档面向后续维护者/二次开发者，描述 ctf-pocgen 的整体架构、各模块职责、关键设计决策与陷阱。**写代码前请先读本文**，尤其是「关键陷阱」一节，避免重蹈已修复的 bug。

## 这是什么工具

输入一个 Java 归档（Spring Boot fat jar / WAR / 普通 JAR / jar-with-dependencies），输出一个**可立即 `mvn compile` 运行的零污染 Maven POC 项目**，用于在本地编写和调试 CTF Java 反序列化利用链。

三条核心原则（不可违背）：

1. **零污染**：所有依赖用 `<scope>system</scope>` + 项目内 `lib/` 目录，**绝不写 `~/.m2`**。
2. **字节级高保真**：题目类逐字节复制到 `challenge-classes.jar`，**不反编译重编译**（避免 `serialVersionUID` 漂移）。
3. **JDK 自动匹配**：从 `MANIFEST.MF` 解析 JDK 版本，写入 `maven.compiler.source/target`，默认兜底 1.8。

Go 编写，编译成单一二进制；带 TUI 和多类型自动识别。

## 包结构与依赖方向

```
ctf-pocgen/
├── main.go / cli.go / tui_run.go   ← 入口层
├── pipeline/    ← 编排：检测→路由→生成（CLI 与 TUI 共用入口）
├── handlers/    ← strategy：各归档类型的处理器
├── analyzer/    ← 叶子：manifest 解析、JDK 检测、类型检测
├── extractor/   ← 叶子：challenge-classes 打包 + lib 复制
├── generator/   ← 叶子：pom/Poc.java/README/脚本模板
├── ide/         ← 叶子：跨平台发现并启动 IntelliJ IDEA
└── tui/         ← 叶子：文件浏览/确认/位置选择/打开IDEA 状态机
```

**依赖方向严格单向**（在 `go vet`/编译期保证无环）：

```
main/cli/tui_run
      ↓
   pipeline
      ↓
   handlers
      ↓
analyzer + extractor + generator   （三者互不依赖，可独立单测）
```

- `analyzer` / `extractor` / `generator` 是叶子包，互不 import，便于单测。
- `pipeline` 是唯一编排者，**CLI 与 TUI 必须都走 `pipeline.Run`**，保证两种调用方式产物完全一致（这是设计约束，别在 CLI 里绕过 pipeline 直连 handlers）。

## 各模块职责与关键 API

### analyzer — 分析层（叶子包）

| 文件 | 职责 | 关键导出 |
|---|---|---|
| `manifest.go` | MANIFEST 解析、JDK 版本归一化 | `DetectJDKFromManifest`, `NormalizeJDKVersion`, `NormalizeCreatedByJDK`, `ReadManifest`, `ManifestHeader`, `LanguageLevel` |
| `jar.go` | jar 结构分析 | `AnalyzeSpringBootJar`, `AnalyzeArchive`, `NotValidZipError`, `NotFatJarError` |
| `detect.go` | 归档类型决策树 | `DetectType`, `ArchiveType`（枚举）, `ListEntriesUnder` |

**类型检测决策树**（`DetectType`，优先级从高到低，勿随意调整顺序）：

```
1. META-INF/application.xml              → TypeEar
2. BOOT-INF/ 且 WEB-INF/（或 loader/lib-provided）→ TypeSpringBootWar
3. BOOT-INF/classes 或 BOOT-INF/lib      → TypeSpringBootJar
4. WEB-INF/classes 或 WEB-INF/lib        → TypeWar
5. 否则                                   → TypePlainJar
```

Spring Boot `Main-Class` 用**前缀匹配** `org.springframework.boot.loader`，覆盖 3.2+ 的 `.loader.launch` 迁移路径（别改成精确匹配，会漏 3.2+）。

### extractor — 提取层（叶子包）

| 函数 | 职责 |
|---|---|
| `ExtractClasses` | 把源 jar 中指定前缀下的条目重新打包为一个新 jar（challenge-classes.jar）。逐字节复制内容、保留时间戳与 external_attr、强制 Deflate。 |
| `ExtractLibJars` | 把源 jar 中指定的 *.jar 条目复制到 libDir，支持 exclude glob。 |
| `CopyJarAsFile` | 整个 jar 逐字节复制（普通 jar 场景）。 |

⚠️ **`ExtractClasses` 用命名返回值 + defer 检查 `zw.Close()`/`out.Close()` 错误**（曾有 bug：Close 错误被 defer 吞掉导致产物静默损坏）。改这个函数时务必保留这个错误传递模式。

### generator — 生成层（叶子包）

| 文件 | 职责 |
|---|---|
| `pom.go` | `MakeArtifactID`、`XMLEscape`、`GeneratePOM` |
| `project.go` | `ProjectInput`（统一渲染输入）、`RenderProject`（写 pom/Poc.java/README/脚本）、`EnsureProjectDirs` |
| `readme.go` + `readme_template.go` | `GenerateReadme` + README 模板 |
| `templates.go` | `Poc.java` / `compile-run.sh` / `compile-run.bat` / `.gitignore` 模板（**逐字字符串**） |
| `writers.go` | `WriteText`(UTF-8+LF) / `WriteBat`(UTF-8**无 BOM**+CRLF) |

**核心设计：`ProjectInput` 收口**。三种 handler 收集完数据后填一个 `ProjectInput`，交给 `RenderProject` 统一生成产物。新增归档类型**不需要**改 generator，只加 handler。

### handlers — strategy 层

`Handler` interface + 三个实现：

| Handler | 处理对象 | classes 来源 | lib 来源 |
|---|---|---|---|
| `SpringBootHandler` | BOOT-INF fat jar | `BOOT-INF/classes/` | `BOOT-INF/lib/*.jar` |
| `WARHandler{IncludeLibProvided}` | WAR / Spring Boot WAR | `WEB-INF/classes/` | `WEB-INF/lib/*.jar`（+ `lib-provided` 若启用） |
| `PlainJarHandler` | 普通 jar / jar-with-dependencies | jar 自身整体复制为 cc.jar | 无（依赖已合并） |

`Options = {ForceJDK, ExcludePatterns, OpenIDEA}` 是 CLI/TUI 透传给 handler 的选项。

### pipeline — 编排层

`Run(jarPath, projectDir, projectName, opts, log)` 是**唯一入口**：
1. 校验文件 → 2. `analyzer.AnalyzeArchive` 取条目+manifest → 3. `analyzer.DetectType` → 4. switch 路由到 handler → 5. `h.Handle(...)` → 6. 若 `opts.OpenIDEA` 调 `ide.Open`（失败只 warn 不中断）。

`Options = handlers.Options`（类型别名，别再单独定义）。

### ide — IDEA 启动层（叶子包）

`Find()` 跨平台发现 IntelliJ IDEA，`Open(projectDir)` 异步启动并打开项目。用 build tags 分平台（`windows.go` / `darwin.go` / `linux.go`）。

**多策略回退发现**（从最通用到最兜底）：
1. PATH 查找 `idea` / `idea.exe`（用户已配置命令行启动器）
2. JetBrains Toolbox 脚本目录（Win `%LOCALAPPDATA%\JetBrains\Toolbox\scripts\` 等）
3. standalone 安装：
   - **Windows**：**查注册表** `HKLM\SOFTWARE\JetBrains\IntelliJ IDEA\<build>` 的默认值（最可靠，IDEA 装非系统盘也能找到）+ glob `Program Files\JetBrains\IntelliJ IDEA*\bin\idea64.exe` 兜底
   - **Linux**：Snap 命令 + `/opt` + Toolbox apps 目录
   - **macOS**：`.app` bundle（用 `open -a` 启动，**不能**直接跑 `idea.sh`）

找不到时返回 `ErrNotFound`，调用方（pipeline）只 warn 不让整个生成失败。

### tui — 交互层（叶子包）

状态机（`tui.go`）：`stateBrowse` → `stateConfirmFile` → `stateChooseLocation`（仅子目录）→ `stateConfirmMore` → `stateConfirmOpenIDEA`（是否打开 IDEA，默认 No）→ `stateDone`。用 Bubble Tea v1 + bubbles/list；确认组件自写（生态无内置）。`Selection{JarPath, OutputDir}` + `OpenIDEA()` 是收集结果，退出 TUI 后由 `tui_run.go` 循环调用 `pipeline.Run` 批量生成（透传 OpenIDEA）。

### 入口层

- `main.go` → `runCLI(os.Args[1:])`
- `cli.go`：`flag` 解析（`--force-jdk` / `--exclude-jars` / `--force` / `--open-idea`）→ 无位置参数则 `runTUI()`，否则走 CLI。`intersperseArgs` 重排参数支持 flag 与位置参数混排。
- `tui_run.go`：`runTUI()` 启动 Bubble Tea，收集 `[]Selection` + `OpenIDEA()` 后批量调 `pipeline.Run`。**非交互终端检测**（`term.IsTerminal`）避免在管道/CI 里卡死。

## 关键陷阱（踩过的坑，勿重犯）

### 1. `.bat` 编码 —— 必须无 BOM、注释全 ASCII

cmd.exe 按 **OEM 代码页**（中文系统 936/GBK）读取/解析 `.bat` 文件内容：

- **UTF-8 BOM**（`EF BB BF`）会被解码成「锘緻」拼到首行 `@echo off` 前，报 `'锘緻echo' 不是内部或外部命令`。→ `WriteBat` **绝不写 BOM**（`TestWriteBat_NoBOM` 守卫）。
- **`chcp 65001` 只改控制台输出代码页，不改 .bat 自身的解析**。所以 `.bat` 里的 REM/echo 中文仍会被 GBK 误读，报 `'锟' is not recognized`。→ `CompileRunBAT` 模板的 REM/echo **全部 ASCII**（`TestWriteBat_NoBOM` 有纯 ASCII 断言）。
- Java 的中文输出靠 `-Dfile.encoding=UTF-8 -Dstdout.encoding=UTF-8`（**两者都要**：现代 JDK 上 `file.encoding` 不控制 `System.out`，只有 `stdout.encoding` 控制）。

改 `CompileRunBAT` 时：注释和 echo 文本用英文；中文只允许出现在 Java 程序自己的 `System.out`（由 JVM 编码 flag 保证）。

### 2. 模板占位符 —— 用 `\x00...\x00`，别用 `${}`/`{}`/反引号

`pom.go` 和 `readme_template.go` 用 `strings.NewReplacer` + `\x00NAME\x00` 这种占位符，**刻意避开**：

- `${project.basedir}`（pom 里是**字面量**，不能用 `${}` 模板）
- Markdown 三重反引号代码块（Go 原始字符串里要小心）
- `text/template` 的 `{{}}`（会把 `${}` 误处理）

要往模板插值，**继续用 `\x00KEY\x00` + NewReplacer**，别换成 `text/template` 或 `fmt.Sprintf`。

### 3. JDK 检测的 Gradle 陷阱 —— `Created-By` 要剔除工具版本

`NormalizeJDKVersion` 抽取所有版本号 token 取首个可识别的。但 `Created-By: Gradle 8.5` 里的 **8.5 是 Gradle 版本号，不是 JDK**（8 恰好映射 1.8，结果"碰巧对"，等 Gradle 升到 9/11/17 就误判）。

→ `DetectJDKFromManifest` 解析 `Created-By` 时走 `NormalizeCreatedByJDK`：先用正则剔除工具名+版本（`gradle/maven/archiver/ant/ivy/sbt`），只在剩余文本找 JDK 信号。`Build-Jdk-Spec`/`Build-Jdk` 不受影响（它们不含工具版本）。

### 4. `ResolveProjectName` 不能对短文件名 panic

曾有 bug：`base[len(base)-4:]` 对 <4 字符文件名越界 panic，且因排在 `os.Stat` 前，非法输入直接 crash。现用 `HasSuffix` 循环剥离后缀。改 `cli.go` 时**保持 `os.Stat` 在 `ResolveProjectName` 之前**（顺序见 `cli.go` 的 `runCLI`）。`TestResolveProjectName` 覆盖了这些边界。

### 5. 字节保真 —— 类文件必须逐字节复制

`ExtractClasses` 用 `f.Open()` → `io.ReadAll` 读解压后的**原始字节**再写入新 jar。**不能**反编译重编译（会改变 `serialVersionUID`）。`TestExtractClasses_VerifyContentByteForByte` 是保真度守卫。

## 开发流程

### 构建 & 测试

```bash
go build -o ctf-pocgen .          # 构建
go test ./...                      # 全量测试（CI 同款）
go vet ./...                       # 静态检查
gofmt -w .                         # 格式化（CI 会检查 gofmt -l 必须为空）
```

CI（`.github/workflows/ci.yml`）在 main 的每次 commit/PR 跑 vet+gofmt+test+多架构构建，`gofmt -l` 非空会 fail。**提交前务必本地跑一遍这三个**。

### 端到端验证

改动后用一个真实的 Spring Boot fat jar 生成项目，跑 `mvn clean compile` + `mvn exec:java`，再验证 `challenge-classes.jar` 字节一致性（生成的 class 文件 SHA-256 应与原 jar 内对应条目一致）。

### 如何新增一种归档类型

1. 在 `analyzer/detect.go` 的决策树加判定分支（注意优先级顺序）+ `ArchiveType` 枚举。
2. 在 `handlers/` 加一个实现 `Handler` interface 的文件，在 `Handle` 里填好 `ProjectInput` 调 `generator.RenderProject`。
3. 在 `pipeline/generate.go` 的 switch 加路由分支。
4. 在 `handlers/` 加端到端测试（参考 `fat_jar_test.go` 用 `makeZip` 构造样本）。

通常**不需要**改 generator —— `ProjectInput` 已抽象了三类差异。

## CI / Release

- `.github/workflows/build.yml`：可复用多架构构建（6 目标：linux/darwin/windows × amd64/arm64），`CGO_ENABLED=0` 纯 Go 交叉编译。
- `.github/workflows/ci.yml`：main commit/PR → vet/fmt/test + 调用 build（产物作 artifact）。
- `.github/workflows/release.yml`：打 `v*` tag → 调用 build → 发 GitHub Release 带二进制。

打 release：`git tag v1.0.0 && git push origin v1.0.0`。
