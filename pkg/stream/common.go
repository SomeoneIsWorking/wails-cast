package stream

import (
	"fmt"
	"os"
	"strings"
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
	path, found := strings.CutPrefix(subtitlePath, "external:")
	if found {
		return path, true
	}
	return "", false
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

func ProcessSubtitles(input *mix.FileOrBuffer, target *mix.TargetFileOrBuffer, ignoreClosedCaptions bool, delaySeconds float64, bold, italic bool) (*mix.FileOrBuffer, error) {
	// Fast path: nothing to transform, return the input untouched.
	if !ignoreClosedCaptions && delaySeconds == 0 && !bold && !italic {
		return input, nil
	}

	// Parse so we can apply IgnoreClosedCaptions, the timing offset and styling.
	subtitles, err := FoBToWebTT(input)
	if err != nil {
		return nil, err
	}
	if ignoreClosedCaptions {
		subtitles = subtitles.RemoveClosedCaptions()
	}
	if delaySeconds != 0 {
		subtitles = subtitles.Shift(delaySeconds)
	}
	if bold || italic {
		subtitles = subtitles.Style(bold, italic)
	}
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
