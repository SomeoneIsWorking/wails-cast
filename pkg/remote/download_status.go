package remote

type DownloadStatus struct {
	Status    string
	Segments  []bool
	URL       string
	MediaType string
	Track     int
}
