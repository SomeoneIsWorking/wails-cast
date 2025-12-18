package stream

import (
	"fmt"
	"os"
	"wails-cast/pkg/mix"
	"wails-cast/pkg/subtitles"
)

func GetEmbeddedIndex(subtitlePath string) (int, bool) {
	var index int
	n, err := fmt.Sscanf(subtitlePath, "embedded:%d", &index)
	if n != 1 || err != nil {
		return 0, false
	}
	return index, true
}

func GetExternalPath(subtitlePath string) (string, bool) {
	var path string
	n, err := fmt.Sscanf(subtitlePath, "external:%s", &path)
	if n != 1 || err != nil {
		return "", false
	}
	return path, true
}

func FoBToWebTT(file *mix.FileOrBuffer) (*subtitles.WebVTTJson, error) {
	if file.IsBuffer {
		return subtitles.Parse(string(file.Buffer))
	} else {
		data, err := os.ReadFile(file.FilePath)
		if err != nil {
			return nil, err
		}
		return subtitles.Parse(string(data))
	}
}

func ProcessSubtitles(input *mix.FileOrBuffer, target *mix.TargetFileOrBuffer, ignoreClosedCaptions bool) (*mix.FileOrBuffer, error) {
	if !ignoreClosedCaptions {
		return input, nil
	}

	// Apply IgnoreClosedCaptions option if requested
	subtitles, err := FoBToWebTT(input)
	if err != nil {
		return nil, err
	}
	subtitles = subtitles.RemoveClosedCaptions()
	webvttString := subtitles.ToWebVTTString()
	if target.IsBuffer {
		return mix.Buffer([]byte(webvttString)), nil
	} else {
		err := os.WriteFile(target.FilePath, []byte(webvttString), 0644)
		if err != nil {
			return nil, err
		}
		return mix.File(target.FilePath), nil
	}

}
