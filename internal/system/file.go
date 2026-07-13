package system

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/zaigie/palworld-server-tool/internal/logger"
)

func GetExecDir() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exePath), nil
}

func CheckIsDir(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return fileInfo.IsDir(), nil
}

func CopyDir(srcDir, dstDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to copy symbolic link %q", path)
		}
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dstDir, relPath)
		if info.IsDir() {
			return os.MkdirAll(dstPath, os.ModePerm)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refusing to copy non-regular file %q", path)
		}
		return CopyFile(path, dstPath)
	})
}

func CopyFile(srcFile, destFile string) error {
	linkInfo, err := os.Lstat(srcFile)
	if err != nil {
		return err
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 || !linkInfo.Mode().IsRegular() {
		return fmt.Errorf("source %q must be a regular file", srcFile)
	}
	input, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	defer input.Close()
	openedInfo, err := input.Stat()
	if err != nil {
		return err
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(linkInfo, openedInfo) {
		return fmt.Errorf("source %q changed while it was opened", srcFile)
	}

	output, err := os.Create(destFile)
	if err != nil {
		return err
	}
	defer output.Close()

	_, err = io.Copy(output, input)
	return err
}

func ZipDir(srcDir, zipFilePath string) error {
	zipFile, err := os.Create(zipFilePath)
	if err != nil {
		return err
	}
	archive := zip.NewWriter(zipFile)

	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to archive symbolic link %q", path)
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("refusing to archive non-regular file %q", path)
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		header.Name = strings.ReplaceAll(relPath, string(os.PathSeparator), "/")

		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if !info.IsDir() {
			if err := copyFileToWriter(writer, path); err != nil {
				return err
			}
		}
		return nil
	})
	if closeErr := archive.Close(); err == nil {
		err = closeErr
	}
	if closeErr := zipFile.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(zipFilePath)
	}
	return err
}

func copyFileToWriter(writer io.Writer, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(writer, file)
	return err
}

func archiveTargetPath(destDir, entryName string) (string, error) {
	destAbs, err := filepath.Abs(destDir)
	if err != nil {
		return "", err
	}
	targetAbs, err := filepath.Abs(filepath.Join(destAbs, filepath.FromSlash(entryName)))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(destAbs, targetAbs)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("archive entry escapes destination: %q", entryName)
	}
	return targetAbs, nil
}

func extractZipFile(file *zip.File, path string) error {
	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	mode := file.Mode().Perm()
	if mode == 0 {
		mode = 0644
	}
	outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, rc)
	return err
}

func UnzipDir(zipFile, destDir string) error {
	reader, err := zip.OpenReader(zipFile)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		path, err := archiveTargetPath(destDir, file.Name)
		if err != nil {
			return err
		}

		if file.FileInfo().IsDir() {
			mode := file.Mode().Perm()
			if mode == 0 {
				mode = 0755
			}
			if err := os.MkdirAll(path, mode); err != nil {
				return err
			}
		} else {
			if !file.Mode().IsRegular() {
				return fmt.Errorf("unsupported zip entry type: %q", file.Name)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			if err := extractZipFile(file, path); err != nil {
				return err
			}
		}
	}
	return nil
}

func CleanAndCreateDir(dirPath string) error {
	if _, err := os.Stat(dirPath); !os.IsNotExist(err) {
		if err := os.RemoveAll(dirPath); err != nil {
			return err
		}
	}
	return os.MkdirAll(dirPath, 0755)
}

// CheckAndCreateDir 检查指定路径的文件夹是否存在，如果不存在则创建它。
func CheckAndCreateDir(path string) error {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		err := os.MkdirAll(path, 0755)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

func GetSavDir(path string) (string, error) {
	var levelFilePath string
	levelFilePath, err := GetLevelSavFilePath(path)
	if err != nil {
		return "", err
	}
	return filepath.Dir(levelFilePath), nil
}

func GetLevelSavFilePath(path string) (string, error) {
	var foundPath string
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Name() == "Level.sav" {
			foundPath = path
			return errors.New("file found")
		}
		return nil
	})

	if foundPath == "" {
		if err != nil && !errors.Is(err, errors.New("file found")) {
			return "", err
		}
		return "", errors.New("file Level.sav not found")
	}

	return foundPath, nil
}

// LimitCacheZipFiles keeps only the latest `n` zip archives in the cache directory
func LimitCacheZipFiles(cacheDir string, n int) {
	files, err := os.ReadDir(cacheDir)
	if err != nil {
		logger.Errorf("Error reading cache directory: %v\n", err)
		return
	}

	zipFiles := []os.DirEntry{}
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".zip" {
			zipFiles = append(zipFiles, file)
		}
	}

	if len(zipFiles) <= n {
		return
	}

	sort.Slice(zipFiles, func(i, j int) bool {
		createTimeI := GetEntryCreateTime(zipFiles[i])
		createTimeJ := GetEntryCreateTime(zipFiles[j])
		return createTimeI.After(createTimeJ)
	})

	for i := n; i < len(zipFiles); i++ {
		path := filepath.Join(cacheDir, zipFiles[i].Name())
		err := os.Remove(path)
		if err != nil {
			logger.Errorf("Failed to delete excess zip file: %v\n", err)
		}
	}
}

type dirInfo struct {
	path       string
	createTime time.Time
}

// LimitCacheDir keeps only the latest `n` directories in the cache directory
func LimitCacheDir(cacheDirPrefix string, n int) error {
	tempDir := os.TempDir()
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		logger.Errorf("LimitCacheDir: error reading temp directory: %v\n", err)
		return err
	}

	var dirs []dirInfo
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(filepath.Base(entry.Name()), cacheDirPrefix) {
			dirPath := filepath.Join(tempDir, entry.Name())
			createTime := GetEntryCreateTime(entry)
			dirs = append(dirs, dirInfo{path: dirPath, createTime: createTime})
		}
	}

	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].createTime.After(dirs[j].createTime)
	})

	if len(dirs) > n {
		for _, dir := range dirs[n:] {
			err := os.RemoveAll(dir.path)
			if err != nil {
				logger.Errorf("LimitCacheDir: error removing directory: %v\n", err)
				return err
			}
		}
	}

	return nil
}

func UnTarGzDir(tarStream io.Reader, destDir string) error {
	gzr, err := gzip.NewReader(tarStream)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()

		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target, err := archiveTargetPath(destDir, header.Name)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.ModePerm); err != nil {
				return err
			}
		case tar.TypeReg:
			targetDir := filepath.Dir(target)
			if err := os.MkdirAll(targetDir, os.ModePerm); err != nil {
				return err
			}
			mode := os.FileMode(header.Mode).Perm()
			if mode == 0 {
				mode = 0644
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}

	return nil
}
