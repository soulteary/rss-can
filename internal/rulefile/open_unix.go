//go:build darwin || linux

package rulefile

import (
	"io/fs"
	"os"
	"time"

	"golang.org/x/sys/unix"
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

// directoryEntry always resolves metadata relative to the opened descriptor,
// independently of d_type support, File.Name or later pathname replacement.
func directoryEntry(parent *os.File, name string) (os.DirEntry, error) {
	var stat unix.Stat_t
	if err := unix.Fstatat(int(parent.Fd()), name, &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return nil, &os.PathError{Op: "stat rule entry", Path: name, Err: err}
	}
	return fs.FileInfoToDirEntry(entryInfo{name: name, stat: stat}), nil
}

type entryInfo struct {
	name string
	stat unix.Stat_t
}

func (i entryInfo) Name() string       { return i.name }
func (i entryInfo) Size() int64        { return i.stat.Size }
func (i entryInfo) ModTime() time.Time { return time.Unix(i.stat.Mtim.Sec, i.stat.Mtim.Nsec) }
func (i entryInfo) IsDir() bool        { return i.Mode().IsDir() }
func (i entryInfo) Sys() any           { return &i.stat }
func (i entryInfo) Mode() os.FileMode {
	mode := os.FileMode(i.stat.Mode & 0777)
	switch i.stat.Mode & unix.S_IFMT {
	case unix.S_IFREG:
	case unix.S_IFDIR:
		mode |= os.ModeDir
	case unix.S_IFLNK:
		mode |= os.ModeSymlink
	case unix.S_IFIFO:
		mode |= os.ModeNamedPipe
	case unix.S_IFSOCK:
		mode |= os.ModeSocket
	case unix.S_IFBLK:
		mode |= os.ModeDevice
	case unix.S_IFCHR:
		mode |= os.ModeDevice | os.ModeCharDevice
	default:
		mode |= os.ModeIrregular
	}
	if i.stat.Mode&unix.S_ISUID != 0 {
		mode |= os.ModeSetuid
	}
	if i.stat.Mode&unix.S_ISGID != 0 {
		mode |= os.ModeSetgid
	}
	if i.stat.Mode&unix.S_ISVTX != 0 {
		mode |= os.ModeSticky
	}
	return mode
}
