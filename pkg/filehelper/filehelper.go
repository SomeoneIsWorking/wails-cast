package filehelper

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func WriteFile(path string, data []byte) error {
	EnsureDir(filepath.Dir(path))
	return os.WriteFile(path, data, 0644)
}

func WriteJson[T any](path string, data *T) error {
	EnsureDir(filepath.Dir(path))
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, bytes, 0644)
	return nil
}

func ReadJson[T any](path string) (*T, error) {
	var result *T
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

func EnsureFileDir(path string) error {
	return EnsureDir(filepath.Dir(path))
}

func EnsureSymlink(src string, target string) error {
	EnsureFileDir(target)
	os.Remove(target)
	return os.Symlink(src, target)
}
