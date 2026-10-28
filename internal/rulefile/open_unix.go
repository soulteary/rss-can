//go:build darwin || linux

package rulefile

import (
	"golang.org/x/sys/unix"
	"os"
)

const directoryFlags = unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC | unix.O_NOFOLLOW | unix.O_NONBLOCK

func openDirectory(path string) (*os.File, error) {
	fd, err := unix.Open(path, directoryFlags, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open rule directory", Path: path, Err: err}
	}
	return os.NewFile(uintptr(fd), path), nil
}
func openChildDirectory(parent *os.File, name string) (*os.File, error) {
	fd, err := unix.Openat(int(parent.Fd()), name, directoryFlags, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open rule group", Path: name, Err: err}
	}
	return os.NewFile(uintptr(fd), name), nil
}
func openChildFile(parent *os.File, name string) (*os.File, error) {
	fd, err := unix.Openat(int(parent.Fd()), name, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open rule", Path: name, Err: err}
	}
	return os.NewFile(uintptr(fd), name), nil
}
