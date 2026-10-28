//go:build !darwin && !linux

package rulefile

import (
	"fmt"
	"os"
)

func unsupported() (*os.File, error) {
	return nil, fmt.Errorf("safe rule loading is supported on Linux and macOS")
}
func openDirectory(string) (*os.File, error)                { return unsupported() }
func openChildDirectory(*os.File, string) (*os.File, error) { return unsupported() }
func openChildFile(*os.File, string) (*os.File, error)      { return unsupported() }

func directoryEntry(*os.File, string) (os.DirEntry, error) {
	return nil, fmt.Errorf("safe rule loading is supported on Linux and macOS")
}
