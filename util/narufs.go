package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var RootDir string

func Path(filename string) string {
	return filepath.Join(RootDir, filename)
}

func SafeSegment(name string) error {
	if name == "" || name == "." || name == ".." {
		return fmt.Errorf("invalid name: %q", name)
	}
	if filepath.Base(name) != name {
		return fmt.Errorf("invalid name: %q", name)
	}
	return nil
}

func SafeJoin(root, rel string) (string, error) {
	var absRoot string
	var full string

	var err error

	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("path must be relative: %q", rel)
	}

	absRoot, err = filepath.Abs(root)
	if err != nil {
		return "", err
	}

	full = filepath.Join(absRoot, rel)
	if full != absRoot && !strings.HasPrefix(full, absRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes working directory: %q", rel)
	}
	return full, nil
}

func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	var file *os.File
	var temp string

	var err error

	file, err = os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp")
	if err != nil {
		return err
	}

	temp = file.Name()

	_, err = file.Write(data)
	if err != nil {
		file.Close()
		os.Remove(temp)

		return err
	}

	err = file.Sync()
	if err != nil {
		file.Close()
		os.Remove(temp)

		return err
	}

	err = file.Close()
	if err != nil {
		os.Remove(temp)

		return err
	}

	err = os.Chmod(temp, perm)
	if err != nil {
		os.Remove(temp)

		return err
	}

	err = os.Rename(temp, path)
	if err != nil {
		os.Remove(temp)

		return err
	}

	return nil
}

func InitFS(dir string) error {
	var abs string

	var err error

	abs, err = filepath.Abs(dir)
	if err != nil {
		return err
	}

	RootDir = abs

	err = os.MkdirAll(RootDir, 0700)
	if err != nil {
		return err
	}

	return os.Chmod(RootDir, 0700)
}
