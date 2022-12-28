package fn

import (
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path"
	"path/filepath"
)

func ScanDirFiles(baseDir string) (files []string) {
	rules, err := os.ReadDir(baseDir)
	if err != nil {
		return nil
	}

	for _, file := range rules {
		files = append(files, filepath.Join(baseDir, file.Name()))
	}

	return files
}

func IsDir(src string) bool {
	p, err := os.Stat(src)
	if err != nil {
		return false
	}
	return p.IsDir()
}

func IsFile(src string) bool {
	p, err := os.Stat(src)
	if err != nil {
		return false
	}
	return p.Mode().IsRegular()
}

func GetFileSHA1(src []byte) string {
	h := sha1.New()
	h.Write(src)
	return hex.EncodeToString(h.Sum(nil))
}

func GetFileContent(file string) []byte {
	buf, err := os.ReadFile(path.Clean(file))
	if err != nil {
		return nil
	}
	return buf
}
