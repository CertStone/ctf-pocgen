
# ctf-pocgen

> 自用项目，不保证积极维护；

> CTF Java 反序列化 POC 脚手架项目生成工具 —— 从一个 Spring Boot fat jar / WAR / 普通 JAR，一键生成可立即编译运行的零污染 Maven POC 项目。

在 CTF Web Java 反序列化题目中，题目通常只提供一个可执行 jar。选手要手动提取 `BOOT-INF/classes` 和 `BOOT-INF/lib`、配置 Maven、对齐 JDK 版本，繁琐且易错（`serialVersionUID` 不匹配、`ClassNotFoundException` 是家常便饭）。本工具把这些全部自动化：**一条命令，生成一个可立即 `mvn compile` 的 POC 项目**。

## ✨ 特性

- **多类型自动识别**：Spring Boot fat jar / Spring Boot WAR / 普通 WAR / 普通 Maven JAR / EAR，自动路由到对应处理逻辑
- **零污染**：全部依赖使用 `<scope>system</scope>` + 项目内 `lib/` 目录，**绝不写入 `~/.m2`**
- **字节级高保真**：题目类逐字节复制到 `challenge-classes.jar`，不做反编译重编译，避免 `serialVersionUID` 漂移
- **JDK 自动检测**：从 `MANIFEST.MF` 按优先级解析 `Build-Jdk-Spec` / `Build-Jdk` / `Created-By`，默认兜底 1.8（CTF 最常见）。能正确区分构建工具自身版本与 JDK 版本。
- **TUI 交互界面**：无参数启动即可在终端里浏览目录、挑选 jar、确认生成
- **高质量模板**：自带 `Poc.java`（含 `getGadget()` / `deserialize()` 骨架与 CC1 注释）、`README.md`、跨平台一键编译运行脚本
- **单一二进制**：Go 编写，编译后无运行时依赖，下载即用

## 📦 安装

### 方式一：下载预编译二进制（推荐）

到 [Releases 页面](../../releases) 下载对应平台的二进制，重命名为 `ctf-pocgen`（或 `ctf-pocgen.exe`），放入 `PATH` 即可。

### 方式二：从源码编译

需要 Go 1.25+。

```bash
git clone https://github.com/CertStone/ctf-pocgen.git
cd ctf-pocgen
go build -o ctf-pocgen .
```

## 🚀 使用

### 命令行模式

```bash
# 最简：自动识别类型，生成 poc-{jar名} 项目
ctf-pocgen challenge.jar

# 指定项目名
ctf-pocgen challenge.jar my-poc

# 强制使用 JDK 11（覆盖自动检测）
ctf-pocgen challenge.jar --force-jdk 11

# 排除部分依赖（逗号分隔的 glob）
ctf-pocgen challenge.jar --exclude-jars 'log4j*,slf4j*'

# 目标目录已存在时强制覆盖
ctf-pocgen challenge.jar --force
```

参数可放在 jar 路径之前或之后（与 Python argparse 风格一致）。

### TUI 模式

不带任何参数启动：

```bash
ctf-pocgen
```

进入交互界面：浏览当前目录的 `.jar/.war/.ear` 文件与子目录 → 选中文件 → 确认处理 →（若在子目录）选择生成位置 → 是否继续挑选 → 批量生成。

### 全部选项

```
ctf-pocgen [jar_path] [project_name] [flags]

Flags:
  -force-jdk string     强制指定 JDK 版本（如 1.8 / 11 / 17），覆盖自动检测
  -exclude-jars string   逗号分隔的 glob 模式，匹配的 lib jar 不引入（如 'log4j*,slf4j*'）
  -force                目标项目目录已存在时强制覆盖（先删除再生成）
```

## 📂 生成的项目结构

```
poc-challenge/
├── pom.xml                  # Maven 配置，全部 system scope（零污染）
├── README.md                # 含编译/运行/IDEA SDK 配置说明
├── lib/                     # 所有依赖
│   ├── challenge-classes.jar # 题目自定义类，字节码与原 jar 完全一致
│   └── *.jar                # 第三方依赖
├── src/main/java/ctf/poc/
│   └── Poc.java             # 反序列化 POC 模板（getGadget / deserialize）
├── src/main/resources/
├── compile-run.bat          # Windows 一键编译运行
├── compile-run.sh           # Linux/macOS 一键编译运行
└── .gitignore
```

生成后：

```bash
cd poc-challenge
mvn clean compile
mvn exec:java            # 运行 ctf.poc.Poc
# 或
bash compile-run.sh     # Linux/macOS
compile-run.bat         # Windows
```

然后编辑 `src/main/java/ctf/poc/Poc.java` 的 `getGadget()` 方法填充利用链。

## 🧩 支持的归档类型

| 类型 | 判定依据 | 处理方式 |
|------|---------|---------|
| Spring Boot Fat JAR | `BOOT-INF/classes` 或 `BOOT-INF/lib` | classes → `challenge-classes.jar`，lib → `lib/` |
| Spring Boot WAR | `WEB-INF/` + Spring Boot loader / `lib-provided` | 同上 + 纳入 `WEB-INF/lib-provided` |
| 普通 WAR | `WEB-INF/classes` 或 `WEB-INF/lib` | `WEB-INF/classes` → cc.jar，`WEB-INF/lib` → `lib/` |
| 普通 JAR | 无 `BOOT-INF`/`WEB-INF` | 该 jar 自身作为 `challenge-classes` 引入 |
| EAR | `META-INF/application.xml` | 提示为容器格式，建议拆出内部模块单独处理 |

Spring Boot 3.2+ 的 `org.springframework.boot.loader.launch.*` 迁移路径也已正确识别。

### 构建工具兼容性

无论题目 jar 是 Maven、Gradle 还是 IntelliJ IDEA Artifacts 打包的，都能正确处理 —— 识别与处理基于归档结构与 `MANIFEST.MF` 字段，不依赖任何构建工具专属元数据。

| 构建工具 | 典型产物 | 处理 |
|---|---|---|
| Maven（assembly / shade / spring-boot） | `jar-with-dependencies`、fat jar、war | ✅ |
| Gradle（shadow / `bootJar` / `war`） | `-all.jar`、Spring Boot fat jar、war | ✅ |
| IntelliJ IDEA Artifacts | 手动配置的 jar | ✅ |

其中 Gradle shadow / Maven assembly / Maven Shade / IntelliJ 的 fat jar 都是把依赖**解包合并**到同一 jar（无嵌套 jar），会被识别为「普通 JAR」并整体作为 `challenge-classes` 引入，**所有类（题目类 + 第三方库类）都能正确导入**。Spring Boot 的 Maven 与 Gradle 产物结构完全一致（`BOOT-INF/classes` + `BOOT-INF/lib`），统一按 fat jar 处理。

### ⚠️ JDK 检测的局限

JDK 版本从 `MANIFEST.MF` 自动解析，但**不同构建工具写入的元数据详略不同**：

- **Maven**：自动写 `Build-Jdk-Spec` / `Build-Jdk` → 能可靠解析 ✅
- **Gradle**：默认 `jar` 任务**不写** `Build-Jdk`（仅 `Created-By: Gradle X.Y`），JDK 检测会兜底到 1.8
- **IntelliJ Artifacts**：manifest 极简，通常**无任何构建元数据** → 兜底 1.8

CTF 场景下兜底 1.8 多数正确；若题目实际是 17/21（如 Gradle/IntelliJ 打的较新项目），用 `--force-jdk` 覆盖即可。工具已正确区分构建工具自身版本与 JDK 版本，不会被 `Gradle 8.5` 之类的版本号误判。

## ⚠️ 关于 IDEA 运行 JDK

`pom.xml` 的 `maven.compiler.source/target` 只决定**编译版本**，不决定**运行 main 时用哪个 JDK**。在 IDEA 中点运行用的是 **Project SDK**（默认取系统默认 JDK）。反序列化 POC 常依赖特定 JDK 内部类（如 CC1 的 `AnnotationInvocationHandler`），用错版本会让链子失效。

设置：**File → Project Structure → Project** → SDK 选为题目要求的 JDK、Language level 对应；Modules → Dependencies 的 Module SDK 同步。详见生成的项目 `README.md`。

## 🏗️ 架构

分层清晰，便于维护与测试：

```
analyzer/    manifest 解析、JDK 检测、类型检测
extractor/   challenge-classes 打包 + lib 复制
generator/   pom/Poc.java/README/脚本模板（逐字复刻）
handlers/    SpringBoot / WAR / PlainJar 三种处理器
pipeline/    检测→路由→生成 编排（CLI 与 TUI 共用入口）
tui/         文件浏览/确认/位置选择 状态机
```

## 🧪 测试

```bash
go test ./...
```

包含 JDK 检测规则、`makeArtifactID`、类型检测决策树、各 handler 端到端、TUI 状态机的单元测试。

## 📄 许可证

[Apache License 2.0](LICENSE)。
