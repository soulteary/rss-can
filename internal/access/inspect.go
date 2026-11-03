package access

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

// Inspect validates an on-disk store without acquiring its writer lock, creating
// files, modifying permissions or changing its authorization version.
func Inspect(path, issuer string) error {
	if !filepath.IsAbs(path) {
		return ErrInvalid
	}
	dir, err := os.Stat(filepath.Dir(path))
	if err != nil {
		return err
	}
	if !dir.IsDir() || dir.Mode().Perm()&0077 != 0 {
		return ErrInvalid
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > maxStoreBytes {
		return ErrInvalid
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var st state
	dec := json.NewDecoder(io.LimitReader(f, maxStoreBytes+1))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&st); err != nil {
		return err
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return ErrInvalid
	}
	if err := validate(st); err != nil {
		return err
	}
	if st.Issuer != "" && st.Issuer != issuer {
		return ErrInvalid
	}
	return nil
}
