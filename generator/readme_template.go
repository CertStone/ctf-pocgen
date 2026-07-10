package generator

// readmeTemplate 是 README.md 模板，逐字复刻自 Python generate_readme 的模板字符串。
// 用 \x00...\x00 作占位符，避免与 markdown 的反引号代码块和 ${...} 字面量冲突。
const readmeTemplate = "# " + "\x00NAME\x00" + ` — CTF 反序列化 POC 项目

> 由 ` + "`ctf-pocgen`" + ` 自动生成 · ` + "\x00TS\x00" + `

## 项目简介

本 Maven 项目基于题目 Spring Boot fat jar 自动生成，用于本地编写和调试
Java 反序列化 POC。

- **源 jar**：` + "`" + "\x00JAR\x00" + "`" + `
- **编译 JDK**：` + "`" + "\x00JDK\x00" + "`" + `
- **challenge 类数量**：` + "\x00CC_CNT\x00" + `
- **第三方依赖数量**：` + "\x00LIB_CNT\x00" + `

### fat jar 元信息

` + "\x00META\x00" + `

### JDK 版本检测来源

` + "\x00DETAIL\x00" + `

## 目录结构

` + "```" + `
` + "\x00NAME\x00" + `/
├── pom.xml                  # Maven 配置，全部使用 system scope（零污染 ~/.m2）
├── README.md
├── lib/                     # 所有依赖（不污染本地 Maven 仓库）
│   ├── ` + "\x00CCF\x00" + `        # 题目自定义类，字节码与原 jar 完全一致
│   └── ...（第三方 jar）
├── src/main/java/ctf/poc/
│   └── Poc.java             # 反序列化 POC 模板（含 getGadget / deserialize）
├── src/main/resources/      # 资源目录（可放投递脚本、字典等）
├── compile-run.bat          # Windows 一键编译运行
├── compile-run.sh           # Linux/macOS 一键编译运行
└── .gitignore
` + "```" + `

## 依赖清单（system scope，不写入 ~/.m2）

` + "\x00LIB_LIST\x00" + `

## 快速开始

### 1. 编译

` + "```bash" + `
mvn clean compile
` + "```" + `

### 2. 运行 POC

` + "```bash" + `
# 方式一：exec 插件（注意 JAVA_HOME 需指向题目要求的 JDK）
mvn exec:java

# 方式二：直接用 java 命令（需先 mvn package 或手工指定 classpath）
mvn -q dependency:build-classpath -Dmdep.outputFile=cp.txt
java -cp "target/classes;$(cat cp.txt)" ctf.poc.Poc
` + "```" + `

或使用项目自带的一键脚本：

` + "```bash" + `
# Linux/macOS
bash compile-run.sh
# Windows
compile-run.bat
` + "```" + `

> ⚠️ **在 IDEA 中运行务必改 Project SDK**
>
> ` + "`pom.xml`" + ` 里的 ` + "`maven.compiler.source/target`" + ` 只决定**用 Java 8 语法/字节码编译**，
> 不决定**运行 main 时用哪个 JDK**。IDEA 点绿色三角运行时用的是 **Project SDK**，
> 默认取你系统的默认 JDK（可能很新），这会让依赖 JDK 内部类的 gadget 链失效。
>
> 设置方法：**File → Project Structure → Project**：
> 1. **SDK** 选为 ` + "`" + "\x00JDK\x00" + "`" + `（没有就 *Add SDK → JDK* 指向本地 ` + "\x00JDK\x00" + ` 安装目录）
> 2. **Language level** 选 ` + "`" + "\x00JDK_LV\x00" + "`" + `
> 3. 同一窗口 **Modules → Dependencies** 里 *Module SDK* 也确认是 ` + "`" + "\x00JDK\x00" + "`" + `
>
> 命令行场景请确保 ` + "`JAVA_HOME`" + ` 指向 ` + "\x00JDK\x00" + ` 后再执行 ` + "`mvn exec:java`" + `。

### 3. 编写利用链

编辑 ` + "`src/main/java/ctf/poc/Poc.java`" + ` 中的 ` + "`getGadget()`" + ` 方法，按模板注释
实现你的 gadget 链。

## 编写 POC 的推荐流程

1. **反编译题目类**：用 jadx / jd-gui / CFR 打开 ` + "`lib/" + "\x00CCF\x00" + "`" + `，
   重点查找 ` + "`InvocationHandler`" + `、` + "`readObject`" + `、` + "`equals/hashCode/compare`" + `、
   动态代理等触发点。
2. **审计第三方库**：检查 ` + "`lib/`" + ` 下有哪些可用 gadget：
   - ` + "`commons-collections 3.x`" + ` → CC1 / CC6 / LazyMap
   - ` + "`commons-collections 4.x`" + ` → CC2 / CC4 / PriorityQueue
   - ` + "`commons-beanutils`" + ` → BeanComparator
   - ` + "`fastjson`" + ` / ` + "`jackson`" + ` → JNDI 注入
   - ` + "`shiro`" + ` → rememberMe AES
3. **检查防御机制**：若存在 ` + "`SerialKiller`" + ` / ` + "`ObjectInputFilter`" + ` 等过滤器，
   需先绕过黑名单（题目 ` + "`lib/serialkiller-*.jar`" + ` 或 ` + "`serialkiller.conf`" + `）。
4. **本地自测**：` + "`getGadget()`" + ` 返回的字节可直接用 ` + "`deserialize()`" + ` 验证触发，
   再用 base64 编码通过 HTTP 接口投递到题目。

## 重要说明

- **零污染**：所有依赖通过 ` + "`<scope>system</scope>`" + ` + ` + "`<systemPath>`" + ` 引入，
  不会向 ` + "`~/.m2`" + ` 安装任何 jar，可放心使用。
- **高保真**：` + "`lib/" + "\x00CCF\x00" + "`" + ` 中的 class 文件字节码与题目 jar 完全一致，
  不会出现 ` + "`serialVersionUID`" + ` 不匹配问题。
- **JDK 匹配**：已按 fat jar 声明的 JDK 版本配置 ` + "`maven.compiler.source/target`" + `，
  如需切换可用 ` + "`--force-jdk`" + ` 重新生成。
- 如果 IDE 报 system scope 依赖无法解析，属于 IDE 已知限制，` + "`mvn`" + ` 命令行编译不受影响。

## 重新生成

` + "```bash" + `
ctf-pocgen ` + "\x00JAR\x00" + ` ` + "\x00NAME\x00" + `
` + "```" + `
`
