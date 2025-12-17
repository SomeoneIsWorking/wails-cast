package cache

import (
	"os"
	"wails-cast/pkg/filehelper"
)

func GetJson[T any](path string, fn func() (*T, error)) (*T, error) {
	value, err := filehelper.ReadJson[T](path)
	if err == nil {
		return value, nil
	}

	// Generate new data
	result, err := fn()
	if err != nil {
		return nil, err
	}

	filehelper.WriteJson(result, path)

	return result, nil
}

func Get(path string, fn func() ([]byte, error)) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return data, nil
	}

	// Generate new data
	result, err := fn()
	if err != nil {
		return nil, err
	}

	filehelper.WriteFile(path, result)

	return result, nil
}
