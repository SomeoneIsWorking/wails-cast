package mediainfo

type SubtitleTrack struct {
	Index    int    `json:"index"`
	Language string `json:"language"`
	Title    string `json:"title"`
}

type VideoTrack struct {
	Index      int    `json:"index"`
	Codec      string `json:"codec"`
	Resolution string `json:"resolution,omitempty"`
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
