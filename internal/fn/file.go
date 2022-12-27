package fn

import (
	"os"
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
