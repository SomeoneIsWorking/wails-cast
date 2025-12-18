package main

import "fmt"

// SendSubtitles sends a subtitle URL to the receiver over the custom namespace
const namespace = "urn:x-cast:com.barishamil.receiver"

func (a *App) sendSubtitles(url string) error {
	if a.App == nil {
		return fmt.Errorf("no chromecast application available")
	}
	return a.App.SendCustom(namespace, "subtitles", url)
}

// SetSubtitleSize instructs the receiver to change subtitle size
func (a *App) SetSubtitleSize(size int) error {
	if a.App == nil {
		return fmt.Errorf("no chromecast application available")
	}
	return a.App.SendCustom(namespace, "subtitleSize", size)
}
