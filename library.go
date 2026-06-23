package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	wails_runtime "github.com/wailsapp/wails/v2/pkg/runtime"

	"wails-cast/pkg/ai"
	"wails-cast/pkg/events"
)

// ─── Organize types ──────────────────────────────────────────────────────────

// OrganizeMove is a single planned file operation (video + optional subtitle
// directory) returned by PreviewOrganize.
type OrganizeMove struct {
	// Absolute paths before and after the move.
	SrcVideo  string `json:"srcVideo"`
	DstVideo  string `json:"dstVideo"`
	// Non-empty when a sibling subtitle directory exists and will be moved.
	SrcSubDir string `json:"srcSubDir"`
	DstSubDir string `json:"dstSubDir"`
	// Human-readable description shown in the UI.
	Description string `json:"description"`
}

// LibraryIdentifyProgress is emitted on "library:identify:progress".
type LibraryIdentifyProgress struct {
	// Total shows being identified.
	Total int `json:"total"`
	// 1-based index of the show being processed right now.
	Current int `json:"current"`
	// Name of the show currently being identified.
	ShowName string `json:"showName"`
	// "running" | "done" | "error"
	Status  string `json:"status"`
	Message string `json:"message"`
}

// ─── Data model ─────────────────────────────────────────────────────────────

// LibraryEpisode represents a single episode file.
type LibraryEpisode struct {
	// Absolute path to the video file.
	Path string `json:"path"`
	// Human-readable name derived from the file name (SxxExx style).
	Name string `json:"name"`
	// Season/episode numbers parsed from the filename (0 = unknown).
	Season  int `json:"season"`
	Episode int `json:"episode"`
	// Whether a subtitle directory already exists next to the video file.
	HasSubtitles bool `json:"hasSubtitles"`
	// EpisodeName is the official episode title from TMDB (empty if unidentified).
	EpisodeName string `json:"episodeName"`
	// Identified is true when TMDB metadata was successfully fetched.
	Identified bool `json:"identified"`
}

// LibrarySeason represents one season folder (or a flat group of episodes
// that share the same season number).
type LibrarySeason struct {
	Name     string           `json:"name"`
	Number   int              `json:"number"`
	Episodes []LibraryEpisode `json:"episodes"`
}

// LibraryShow represents a TV show (or movie folder).
type LibraryShow struct {
	Name    string          `json:"name"`
	Path    string          `json:"path"`
	Seasons []LibrarySeason `json:"seasons"`
	// TMDB / external IDs — populated after IdentifyLibrary is called.
	TMDBID int    `json:"tmdbId"`
	IMDBID string `json:"imdbId"`
	// Year is the first-air-date year from TMDB.
	Year int `json:"year"`
	// Identified is true when TMDB metadata was successfully fetched.
	Identified bool `json:"identified"`
}

// LibraryScanResult is the full result returned by ScanLibrary.
type LibraryScanResult struct {
	RootPath string        `json:"rootPath"`
	Shows    []LibraryShow `json:"shows"`
}

// ─── Season-batch translation ────────────────────────────────────────────────

// SeasonTranslateProgress is emitted on event "library:translate:progress".
type SeasonTranslateProgress struct {
	ShowName       string `json:"showName"`
	SeasonName     string `json:"seasonName"`
	TargetLanguage string `json:"targetLanguage"`
	TotalEpisodes  int    `json:"totalEpisodes"`
	// 1-based index of the episode currently being translated (0 = not started).
	CurrentEpisode int `json:"currentEpisode"`
	// "running" | "done" | "cancelled" | "error"
	Status  string `json:"status"`
	Message string `json:"message"`
}

// ─── Regexp helpers ──────────────────────────────────────────────────────────

// Matches SxxExx (case-insensitive) e.g. "S01E03", "s1e12".
var reSeasonEpisode = regexp.MustCompile(`(?i)[Ss](\d+)[Ee](\d+)`)

// Matches "Season N" in a folder name.
var reFolderSeason = regexp.MustCompile(`(?i)season\s*(\d+)`)

// videoExtensions is the set of recognised video file extensions.
var videoExtensions = map[string]bool{
	".mp4": true, ".mkv": true, ".webm": true,
	".avi": true, ".mov": true, ".flv": true,
	".m4v": true,
}

// ─── Scanner ─────────────────────────────────────────────────────────────────

// parseEpisodeNumbers extracts (season, episode) from a filename using the
// SxxExx convention. Returns (0, 0) when the pattern is not found.
func parseEpisodeNumbers(filename string) (season, episode int) {
	m := reSeasonEpisode.FindStringSubmatch(filename)
	if m == nil {
		return 0, 0
	}
	s, _ := strconv.Atoi(m[1])
	e, _ := strconv.Atoi(m[2])
	return s, e
}

// cleanShowName strips trailing " - Season N" or "Season N" suffix from a
// folder name to derive a clean show title.
func cleanShowName(name string) string {
	re := regexp.MustCompile(`(?i)\s*-?\s*season\s*\d+\s*$`)
	clean := re.ReplaceAllString(name, "")
	return strings.TrimSpace(clean)
}

// cleanEpisodeName strips the show-title prefix (if present) and the season/
// episode tag from a filename, returning a tidy label like "E03 – The Title".
func cleanEpisodeName(filename string) string {
	// Remove extension.
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	// If the file contains SxxExx, build a short label from the remainder.
	m := reSeasonEpisode.FindStringSubmatchIndex(name)
	if m == nil {
		return name
	}
	// Take everything after "SxxExx" as the title (trim leading " - ").
	suffix := strings.TrimLeft(name[m[1]:], " -–")
	epLabel := strings.ToUpper(name[m[0]:m[1]])
	if suffix != "" {
		return epLabel + " – " + suffix
	}
	return epLabel
}

// subtitleDirExists returns true when a sibling directory with the same base
// name as the video exists (the app's subtitle convention).
func subtitleDirExists(videoPath string) bool {
	base := strings.TrimSuffix(videoPath, filepath.Ext(videoPath))
	info, err := os.Stat(base)
	return err == nil && info.IsDir()
}

// scanSeasonDir scans a single directory (season folder) for video files and
// returns the episodes sorted by episode number.
func scanSeasonDir(dir string) []LibraryEpisode {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var episodes []LibraryEpisode
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !videoExtensions[ext] {
			continue
		}
		absPath := filepath.Join(dir, e.Name())
		s, ep := parseEpisodeNumbers(e.Name())
		episodes = append(episodes, LibraryEpisode{
			Path:         absPath,
			Name:         cleanEpisodeName(e.Name()),
			Season:       s,
			Episode:      ep,
			HasSubtitles: subtitleDirExists(absPath),
		})
	}

	sort.Slice(episodes, func(i, j int) bool {
		if episodes[i].Season != episodes[j].Season {
			return episodes[i].Season < episodes[j].Season
		}
		return episodes[i].Episode < episodes[j].Episode
	})
	return episodes
}

// scanShowDir analyses a show directory. It looks for season sub-folders
// (matching "Season N" in the name) and — if none are found — treats the
// directory itself as a single flat season.
func scanShowDir(showDir string) LibraryShow {
	show := LibraryShow{
		Name: cleanShowName(filepath.Base(showDir)),
		Path: showDir,
	}

	entries, err := os.ReadDir(showDir)
	if err != nil {
		return show
	}

	// First pass: find season sub-dirs.
	var seasonDirs []struct {
		path   string
		number int
		name   string
	}
	var hasSeasonDirs bool
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		m := reFolderSeason.FindStringSubmatch(e.Name())
		if m != nil {
			n, _ := strconv.Atoi(m[1])
			seasonDirs = append(seasonDirs, struct {
				path   string
				number int
				name   string
			}{
				path:   filepath.Join(showDir, e.Name()),
				number: n,
				name:   e.Name(),
			})
			hasSeasonDirs = true
		}
	}

	if hasSeasonDirs {
		sort.Slice(seasonDirs, func(i, j int) bool {
			return seasonDirs[i].number < seasonDirs[j].number
		})
		for _, sd := range seasonDirs {
			eps := scanSeasonDir(sd.path)
			if len(eps) > 0 {
				show.Seasons = append(show.Seasons, LibrarySeason{
					Name:     sd.name,
					Number:   sd.number,
					Episodes: eps,
				})
			}
		}
	} else {
		// No season sub-dirs – treat the show dir as a single season.
		eps := scanSeasonDir(showDir)
		if len(eps) > 0 {
			// Try to determine season number from first episode.
			sNum := 1
			if len(eps) > 0 && eps[0].Season > 0 {
				sNum = eps[0].Season
			}
			show.Seasons = append(show.Seasons, LibrarySeason{
				Name:     fmt.Sprintf("Season %d", sNum),
				Number:   sNum,
				Episodes: eps,
			})
		}
	}

	return show
}

// ScanLibrary recursively scans rootPath for TV shows / movies. Each
// immediate child directory is treated as a show root; deeper nesting is
// handled by scanShowDir.
func (a *App) ScanLibrary(rootPath string) (*LibraryScanResult, error) {
	result, err := scanLibraryRoot(rootPath)
	if err != nil {
		return nil, err
	}

	// Persist the library root in settings so the frontend can restore it.
	settings := a.settingsStore.Get()
	settings.LibraryRoot = rootPath
	_ = a.settingsStore.Update(*settings)

	return result, nil
}

// scanLibraryRoot performs a read-only recursive scan of the library root and
// returns the show/season/episode tree. It has no side effects (it does not
// persist settings), so it is safe to call from non-interactive callers such
// as the HTTP remote API.
func scanLibraryRoot(rootPath string) (*LibraryScanResult, error) {
	if rootPath == "" {
		return nil, fmt.Errorf("rootPath must not be empty")
	}

	info, err := os.Stat(rootPath)
	if err != nil {
		return nil, fmt.Errorf("cannot access library root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("rootPath is not a directory")
	}

	entries, err := os.ReadDir(rootPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read library root: %w", err)
	}

	result := &LibraryScanResult{RootPath: rootPath}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		showPath := filepath.Join(rootPath, e.Name())
		show := scanShowDir(showPath)
		// Only include entries that have at least one episode.
		totalEps := 0
		for _, s := range show.Seasons {
			totalEps += len(s.Episodes)
		}
		if totalEps > 0 {
			result.Shows = append(result.Shows, show)
		}
	}

	return result, nil
}

// ListLibraryItems implements the LibraryLister interface used by the HTTP
// remote API. It scans the configured library root and flattens every episode
// into a flat list of LibraryItems so companion apps can browse and select
// content. Returns an empty list (not an error) when no library is configured.
func (a *App) ListLibraryItems() ([]LibraryItem, error) {
	root := a.settingsStore.Get().LibraryRoot
	if root == "" {
		return []LibraryItem{}, nil
	}

	result, err := scanLibraryRoot(root)
	if err != nil {
		return nil, err
	}

	items := make([]LibraryItem, 0)
	for _, show := range result.Shows {
		for _, season := range show.Seasons {
			for _, ep := range season.Episodes {
				name := ep.Name
				if ep.EpisodeName != "" {
					name = fmt.Sprintf("%s - S%02dE%02d - %s", show.Name, ep.Season, ep.Episode, ep.EpisodeName)
				}
				items = append(items, LibraryItem{
					ID:   ep.Path,
					Name: name,
					Path: ep.Path,
				})
			}
		}
	}

	return items, nil
}

// OpenLibraryFolderDialog shows a native folder picker and returns the chosen
// path (empty string if cancelled).
func (a *App) OpenLibraryFolderDialog() (string, error) {
	return wails_runtime.OpenDirectoryDialog(a.ctx, wails_runtime.OpenDialogOptions{
		Title: "Select Library Folder",
	})
}

// ─── Season-batch translation ────────────────────────────────────────────────

// libraryTranslateMu guards the library-level batch translation (separate from
// the single-file translationMu so they don't interfere).
var libraryTranslateMu sync.Mutex
var libraryTranslateCancel context.CancelFunc

// TranslateSeason translates all episodes in a season sequentially, one at a
// time. Progress is reported via the "library:translate:progress" event.
// The function returns immediately; work happens in a background goroutine.
func (a *App) TranslateSeason(
	showName string,
	seasonName string,
	episodePaths []string,
	targetLanguage string,
) error {
	if len(episodePaths) == 0 {
		return fmt.Errorf("no episodes provided")
	}
	if targetLanguage == "" {
		return fmt.Errorf("target language is required")
	}

	// Check / retrieve API key the same way TranslateExportedSubtitles does.
	settings := a.settingsStore.Get()
	var apiKey, model, baseURL string
	switch settings.LLMProvider {
	case ai.ProviderOpenAICompat:
		apiKey = settings.LLMApiKey
		model = settings.LLMModel
		baseURL = settings.LLMBaseURL
		if apiKey == "" {
			return fmt.Errorf("openai-compat API key is required. Please set it in Settings")
		}
		if baseURL == "" {
			return fmt.Errorf("openai-compat base URL is required. Please set it in Settings")
		}
		if model == "" {
			return fmt.Errorf("openai-compat model is required. Please set it in Settings")
		}
	default:
		apiKey = settings.LLMApiKey
		if apiKey == "" {
			if key, err := ai.LoadOpenCodeAPIKey(); err == nil {
				apiKey = key
			}
		}
		if apiKey == "" {
			return fmt.Errorf("opencode API key is required. Please set it in Settings or in opencode's auth.json")
		}
		model = settings.LLMModel
		baseURL = ai.OpenCodeBaseURL
	}

	libraryTranslateMu.Lock()
	if libraryTranslateCancel != nil {
		libraryTranslateMu.Unlock()
		return fmt.Errorf("a season translation is already in progress")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	libraryTranslateCancel = cancel
	libraryTranslateMu.Unlock()

	emitProgress := func(current int, status, message string) {
		events.Emit("library:translate:progress", SeasonTranslateProgress{
			ShowName:       showName,
			SeasonName:     seasonName,
			TargetLanguage: targetLanguage,
			TotalEpisodes:  len(episodePaths),
			CurrentEpisode: current,
			Status:         status,
			Message:        message,
		})
	}

	go func() {
		defer func() {
			libraryTranslateMu.Lock()
			libraryTranslateCancel = nil
			libraryTranslateMu.Unlock()
			cancel()
		}()

		emitProgress(0, "running", "Starting season translation…")

		for i, ep := range episodePaths {
			// Check for cancellation between episodes.
			select {
			case <-ctx.Done():
				emitProgress(i, "cancelled", "Translation cancelled by user")
				return
			default:
			}

			emitProgress(i+1, "running", fmt.Sprintf("Translating episode %d of %d…", i+1, len(episodePaths)))

			req := ai.Request{
				FileNameOrURL:  ep,
				TargetLanguage: targetLanguage,
				APIKey:         apiKey,
				Model:          model,
				BaseURL:        baseURL,
				PromptTemplate: settings.TranslatePromptTemplate,
				MaxSamples:     settings.MaxSubtitleSamples,
			}

			// Use a per-episode timeout to avoid hanging forever.
			epCtx, epCancel := context.WithTimeout(ctx, 30*time.Minute)
			_, err := ai.TranslateForFile(epCtx, req)
			epCancel()

			if err != nil {
				if ctx.Err() == context.Canceled {
					emitProgress(i+1, "cancelled", "Translation cancelled by user")
					return
				}
				// Non-fatal: log the error and continue with the next episode.
				logger.Warn("Season translate: episode failed", "episode", ep, "error", err)
				emitProgress(i+1, "running", fmt.Sprintf("Episode %d failed (%s), continuing…", i+1, err.Error()))
				continue
			}
		}

		emitProgress(len(episodePaths), "done", "Season translation complete!")
	}()

	return nil
}

// CancelSeasonTranslation cancels an in-progress season batch translation.
func (a *App) CancelSeasonTranslation() {
	libraryTranslateMu.Lock()
	cancel := libraryTranslateCancel
	libraryTranslateMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// ─── TMDB identification ─────────────────────────────────────────────────────

// IdentifyLibrary enriches a scan result with TMDB metadata (show IDs, episode
// names). It runs in the foreground and emits "library:identify:progress" events
// as it processes each show. The enriched result is returned directly so the
// frontend can replace its cached copy.
//
// If no TMDB API key is configured the result is returned unchanged.
func (a *App) IdentifyLibrary(result *LibraryScanResult) (*LibraryScanResult, error) {
	if result == nil {
		return nil, fmt.Errorf("result must not be nil")
	}

	apiKey := a.settingsStore.Get().TMDBApiKey
	if apiKey == "" {
		// Graceful degradation: return the scan result as-is.
		return result, nil
	}

	total := len(result.Shows)
	emitIdentProgress := func(current int, showName, status, message string) {
		events.Emit("library:identify:progress", LibraryIdentifyProgress{
			Total:    total,
			Current:  current,
			ShowName: showName,
			Status:   status,
			Message:  message,
		})
	}

	emitIdentProgress(0, "", "running", fmt.Sprintf("Identifying %d show(s)…", total))

	for i, show := range result.Shows {
		emitIdentProgress(i+1, show.Name, "running", fmt.Sprintf("Looking up %q…", show.Name))

		entry := fetchTMDBShow(apiKey, show.Name)
		if entry == nil {
			logger.Info("TMDB: show not identified", "show", show.Name)
			continue
		}

		result.Shows[i].TMDBID = entry.TMDBID
		result.Shows[i].IMDBID = entry.IMDBID
		result.Shows[i].Year = entry.Year
		result.Shows[i].Identified = true

		// Enrich each episode with TMDB episode names.
		for si, season := range result.Shows[i].Seasons {
			epNames := fetchTMDBSeason(apiKey, entry, season.Number)
			if epNames == nil {
				continue
			}
			for ei, ep := range season.Episodes {
				if name, ok := epNames[ep.Episode]; ok && name != "" {
					result.Shows[i].Seasons[si].Episodes[ei].EpisodeName = name
					result.Shows[i].Seasons[si].Episodes[ei].Identified = true
				}
			}
		}
	}

	emitIdentProgress(total, "", "done", "Identification complete!")
	return result, nil
}

// ─── Organize (preview + execute) ───────────────────────────────────────────

// PreviewOrganize builds the list of file moves needed to reorganise the
// library into the canonical layout without touching the filesystem.
//
// Target layout (relative to rootPath):
//
//	<Show Name>/Season NN/<XX> - <Episode Name>.<ext>
//
// Only episodes that are identified (have an EpisodeName) are included.
// Unidentified episodes are skipped.
func (a *App) PreviewOrganize(result *LibraryScanResult) ([]OrganizeMove, error) {
	if result == nil {
		return nil, fmt.Errorf("result must not be nil")
	}
	if result.RootPath == "" {
		return nil, fmt.Errorf("rootPath must not be empty")
	}

	var plan []OrganizeMove

	for _, show := range result.Shows {
		// Use the TMDB name if available (show.Name is already cleaned).
		showDir := sanitizeFilename(show.Name)

		for _, season := range show.Seasons {
			seasonDir := fmt.Sprintf("Season %02d", season.Number)

			for _, ep := range season.Episodes {
				// Skip unidentified episodes.
				if !ep.Identified || ep.EpisodeName == "" {
					continue
				}

				ext := filepath.Ext(ep.Path)
				epTitle := sanitizeFilename(ep.EpisodeName)
				fileName := fmt.Sprintf("%02d - %s%s", ep.Episode, epTitle, ext)

				dstVideo := filepath.Join(result.RootPath, showDir, seasonDir, fileName)

				// Skip if source and destination are the same.
				if ep.Path == dstVideo {
					continue
				}

				move := OrganizeMove{
					SrcVideo:    ep.Path,
					DstVideo:    dstVideo,
					Description: fmt.Sprintf("%s / %s / %s", show.Name, seasonDir, fileName),
				}

				// Check for sibling subtitle directory.
				srcSubDir := strings.TrimSuffix(ep.Path, ext)
				if info, err := os.Stat(srcSubDir); err == nil && info.IsDir() {
					subDirName := fmt.Sprintf("%02d - %s", ep.Episode, epTitle)
					move.SrcSubDir = srcSubDir
					move.DstSubDir = filepath.Join(result.RootPath, showDir, seasonDir, subDirName)
				}

				plan = append(plan, move)
			}
		}
	}

	return plan, nil
}

// OrganizeLibrary executes a move plan previously returned by PreviewOrganize.
// It creates destination directories as needed, moves each video (and its
// sibling subtitle directory if present), and skips moves where the destination
// already exists. Nothing is deleted.
func (a *App) OrganizeLibrary(plan []OrganizeMove) error {
	if len(plan) == 0 {
		return nil
	}

	for _, m := range plan {
		// Skip if destination already exists to avoid accidental overwrites.
		if _, err := os.Stat(m.DstVideo); err == nil {
			logger.Info("Organize: destination exists, skipping", "dst", m.DstVideo)
			continue
		}

		// Ensure destination directory exists.
		if err := os.MkdirAll(filepath.Dir(m.DstVideo), 0755); err != nil {
			return fmt.Errorf("cannot create directory for %s: %w", m.DstVideo, err)
		}

		// Move video file.
		if err := os.Rename(m.SrcVideo, m.DstVideo); err != nil {
			return fmt.Errorf("moving %s → %s: %w", m.SrcVideo, m.DstVideo, err)
		}
		logger.Info("Organize: moved video", "src", m.SrcVideo, "dst", m.DstVideo)

		// Move subtitle directory if applicable.
		if m.SrcSubDir != "" && m.DstSubDir != "" {
			if _, err := os.Stat(m.DstSubDir); err == nil {
				logger.Info("Organize: subtitle dir destination exists, skipping", "dst", m.DstSubDir)
				continue
			}
			if err := os.Rename(m.SrcSubDir, m.DstSubDir); err != nil {
				// Non-fatal: log and continue.
				logger.Warn("Organize: failed to move subtitle dir", "src", m.SrcSubDir, "dst", m.DstSubDir, "error", err)
			} else {
				logger.Info("Organize: moved subtitle dir", "src", m.SrcSubDir, "dst", m.DstSubDir)
			}
		}
	}

	return nil
}
