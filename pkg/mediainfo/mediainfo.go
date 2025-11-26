package mediainfo

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

type SubtitleTrack struct {
	Index    int    `json:"index"`
	Language string `json:"language"`
	Title    string `json:"title"`
	Codec    string `json:"codec"`
}

type VideoTrack struct {
	Index      int    `json:"index"`
	Codec      string `json:"codec"`
	Resolution string `json:"resolution,omitempty"`
	Type       string // "AUDIO" or "VIDEO"
	URI        string
	GroupID    string
	Name       string
	IsDefault  bool
	Bandwidth  int
	Codecs     string
}

type AudioTrack struct {
	Index     int    `json:"index"`
	Language  string `json:"language"`
	Codec     string `json:"codec"`
	Type      string // "AUDIO" or "VIDEO"
	URI       string
	GroupID   string
	Name      string
	IsDefault bool
	Bandwidth int
	Codecs    string
}

type MediaTrackInfo struct {
	VideoTracks    []VideoTrack    `json:"videoTracks"`
	AudioTracks    []AudioTrack    `json:"audioTracks"`
	SubtitleTracks []SubtitleTrack `json:"subtitleTracks"`
}

// GetSubtitleTracks gets subtitle tracks from a video file using ffprobe
func GetSubtitleTracks(videoPath string) ([]SubtitleTrack, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "s",
		"-show_entries", "stream=index:stream_tags=language,title:stream=codec_name",
		"-of", "json",
		videoPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var result struct {
		Streams []struct {
			Index     int    `json:"index"`
			CodecName string `json:"codec_name"`
			Tags      struct {
				Language string `json:"language"`
				Title    string `json:"title"`
			} `json:"tags"`
		} `json:"streams"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, err
	}

	tracks := make([]SubtitleTrack, 0, len(result.Streams))
	for i, stream := range result.Streams {
		track := SubtitleTrack{
			Index:    i,
			Language: stream.Tags.Language,
			Title:    stream.Tags.Title,
			Codec:    stream.CodecName,
		}
		tracks = append(tracks, track)
	}

	return tracks, nil
}

// GetMediaTrackInfo gets all track information for a media file using ffprobe
func GetMediaTrackInfo(mediaPath string) (*MediaTrackInfo, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "stream=index,codec_type,codec_name,width,height:stream_tags=language,title",
		"-of", "json",
		mediaPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var result struct {
		Streams []struct {
			Index     int    `json:"index"`
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			Tags      struct {
				Language string `json:"language"`
				Title    string `json:"title"`
			} `json:"tags"`
		} `json:"streams"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, err
	}

	info := &MediaTrackInfo{
		VideoTracks:    make([]VideoTrack, 0),
		AudioTracks:    make([]AudioTrack, 0),
		SubtitleTracks: make([]SubtitleTrack, 0),
	}

	videoIdx := 0
	audioIdx := 0
	subtitleIdx := 0

	for _, stream := range result.Streams {
		switch stream.CodecType {
		case "video":
			resolution := ""
			if stream.Width > 0 && stream.Height > 0 {
				resolution = fmt.Sprintf("%dx%d", stream.Width, stream.Height)
			}
			info.VideoTracks = append(info.VideoTracks, VideoTrack{
				Index:      videoIdx,
				Codec:      stream.CodecName,
				Resolution: resolution,
			})
			videoIdx++
		case "audio":
			info.AudioTracks = append(info.AudioTracks, AudioTrack{
				Index:    audioIdx,
				Language: stream.Tags.Language,
				Codec:    stream.CodecName,
			})
			audioIdx++
		case "subtitle":
			info.SubtitleTracks = append(info.SubtitleTracks, SubtitleTrack{
				Index:    subtitleIdx,
				Language: stream.Tags.Language,
				Title:    stream.Tags.Title,
				Codec:    stream.CodecName,
			})
			subtitleIdx++
		}
	}

	return info, nil
}
