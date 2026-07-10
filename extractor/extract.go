// Package extractor 负责从源 jar 中提取题目类与第三方依赖。
//
// 对应原 Python 版 create-ctf-poc.py 的 Extractor 层：
//   - ExtractClasses: 把指定前缀下的全部条目重新打包为一个 jar（challenge-classes.jar）
//   - ExtractLibJars: 把指定前缀下的 *.jar 复制到目标目录
package extractor

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// 类字节复制缓冲区大小（对应 Python shutil.copyfileobj length=1MiB）。
const copyChunkSize = 1 << 20 // 1 MiB

// ExtractClasses 把源 jar 中 classesEntries 列出的条目重新打包到 outJarPath。
//
// 对应 Python 的 extract_challenge_classes：
//   - 去掉来源前缀（classesPrefix，如 "BOOT-INF/classes/"），得到 jar 内标准路径
//   - 复制原条目的时间戳与 external_attr
//   - 强制使用 ZIP_DEFLATED（在 Go 中即 Deflate）
//   - 资源文件（非 .class）一并打包
//   - 字节内容逐字节复制（f.Open() 读取解压后的原始字节）
//
// classesPrefix 用于从每个条目名剥离前缀。
// 返回写入的条目数。
func ExtractClasses(jarPath string, classesEntries []string, classesPrefix, outJarPath string) (n int, err error) {
	// 确保父目录存在
	if dir := filepath.Dir(outJarPath); dir != "" {
		if err = os.MkdirAll(dir, 0o755); err != nil {
			return 0, err
		}
	}

	r, err := zip.OpenReader(jarPath)
	if err != nil {
		return 0, err
	}
	defer r.Close()

	out, err := os.Create(outJarPath)
	if err != nil {
		return 0, err
	}
	// zip.Writer.Close() 会写中央目录并 flush，是产物完整性的关键步骤，
	// 错误（如磁盘满）必须返回，不能被 defer 吞掉。
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()

	zw := zip.NewWriter(out)
	defer func() {
		cerr := zw.Close()
		if err == nil {
			err = cerr
		}
	}()

	// 建立条目名 -> *zip.File 的索引，便于按 classesEntries 顺序读取。
	index := make(map[string]*zip.File, len(r.File))
	for _, f := range r.File {
		index[f.Name] = f
	}

	for _, entry := range classesEntries {
		f, ok := index[entry]
		if !ok {
			continue
		}
		inner := strings.TrimPrefix(entry, classesPrefix)
		if inner == "" {
			continue
		}

		// 读取原始解压字节
		rc, oerr := f.Open()
		if oerr != nil {
			return n, oerr
		}
		data, rerr := io.ReadAll(rc)
		rc.Close()
		if rerr != nil {
			return n, rerr
		}

		// 构造新的 FileHeader，复制时间戳与 external_attr，强制 Deflate。
		fh := f.FileHeader
		fh.Name = inner
		fh.Method = zip.Deflate

		w, werr := zw.CreateHeader(&fh)
		if werr != nil {
			return n, werr
		}
		if _, werr = w.Write(data); werr != nil {
			return n, werr
		}
		n++
	}
	return n, nil
}

// ExtractLibJars 把源 jar 中 libEntries 列出的 jar 条目复制到 libDir，
// 文件名取条目的 basename。
//
// 对应 Python 的 extract_lib_jars：
//   - excludePatterns 做路径模式匹配（大小写敏感，与 POSIX 上 fnmatch 一致）
//   - 按 1MiB chunk 复制
//
// 返回 (已复制, 已跳过) 两个文件名列表。
func ExtractLibJars(jarPath string, libEntries []string, libDir string, excludePatterns []string) (copied, skipped []string, err error) {
	if mkErr := os.MkdirAll(libDir, 0o755); mkErr != nil {
		return nil, nil, mkErr
	}

	r, err := zip.OpenReader(jarPath)
	if err != nil {
		return nil, nil, err
	}
	defer r.Close()

	index := make(map[string]*zip.File, len(r.File))
	for _, f := range r.File {
		index[f.Name] = f
	}

	for _, entry := range libEntries {
		name := filepath.Base(entry)
		if excluded(name, excludePatterns) {
			skipped = append(skipped, name)
			continue
		}
		f, ok := index[entry]
		if !ok {
			continue
		}
		dst := filepath.Join(libDir, name)
		if cErr := copyZipEntryToFile(f, dst); cErr != nil {
			return copied, skipped, cErr
		}
		copied = append(copied, name)
	}
	return copied, skipped, nil
}

// CopyJarAsFile 把整个源 jar 文件复制为 dstPath（用于普通 jar：自身作为 challenge-classes）。
func CopyJarAsFile(srcJarPath, dstPath string) error {
	if dir := filepath.Dir(dstPath); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	in, err := os.Open(srcJarPath)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.CopyBuffer(out, in, make([]byte, copyChunkSize))
	return err
}

// excluded 判断 name 是否匹配任一 glob 排除模式。
// 与 POSIX 上 Python fnmatch 一致：大小写敏感。
func excluded(name string, patterns []string) bool {
	for _, p := range patterns {
		if ok, _ := filepath.Match(p, name); ok {
			return true
		}
	}
	return false
}

func copyZipEntryToFile(f *zip.File, dstPath string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.CopyBuffer(out, rc, make([]byte, copyChunkSize))
	return err
}
