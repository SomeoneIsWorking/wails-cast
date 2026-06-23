package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ─── TMDB API types ──────────────────────────────────────────────────────────

type tmdbSearchResult struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	OriginalName string `json:"original_name"`
	FirstAirDate string `json:"first_air_date"`
}

type tmdbSearchResponse struct {
	Results []tmdbSearchResult `json:"results"`
}

type tmdbExternalIDs struct {
	IMDbID string `json:"imdb_id"`
}

type tmdbEpisode struct {
	EpisodeNumber int    `json:"episode_number"`
	Name          string `json:"name"`
	Overview      string `json:"overview"`
}

type tmdbSeasonDetail struct {
	Episodes []tmdbEpisode `json:"episodes"`
}

// ─── Cached show metadata ─────────────────────────────────────────────────────

// tmdbShowCache holds per-show TMDB data to avoid re-fetching.
type tmdbShowCache struct {
	TMDBID  int
	IMDBID  string
	Year    int
	Seasons map[int]map[int]string // season → episode → name
}

// tmdbCache is an in-process cache keyed by cleanShowName.
var (
	tmdbCacheMu sync.RWMutex
	tmdbCache   = make(map[string]*tmdbShowCache)
)

// ─── Helpers ─────────────────────────────────────────────────────────────────

var tmdbClient = &http.Client{Timeout: 15 * time.Second}

// tmdbGet performs a GET to the TMDB v3 API and decodes the JSON response.
func tmdbGet(apiKey, path string, query url.Values, out any) error {
	base := "https://api.themoviedb.org/3"
	u, err := url.Parse(base + path)
	if err != nil {
		return err
	}
	if query == nil {
		query = url.Values{}
	}
	query.Set("api_key", apiKey)
	u.RawQuery = query.Encode()

	resp, err := tmdbClient.Get(u.String())
	if err != nil {
		return fmt.Errorf("tmdb request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading tmdb response: %w", err)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("tmdb returned %d: %s", resp.StatusCode, string(body))
	}
	return json.Unmarshal(body, out)
}

// reYear matches a 4-digit year in parentheses, e.g. "(2023)".
var reYear = regexp.MustCompile(`\((\d{4})\)`)

// extractYearFromName pulls a year out of the show name and returns
// (cleanedName, year).  Year is 0 if not found.
func extractYearFromName(name string) (string, int) {
	m := reYear.FindStringSubmatchIndex(name)
	if m == nil {
		return strings.TrimSpace(name), 0
	}
	year := 0
	fmt.Sscanf(name[m[2]:m[3]], "%d", &year)
	cleaned := strings.TrimSpace(name[:m[0]])
	return cleaned, year
}

// sanitizeFilename replaces characters that are illegal or problematic on
// common filesystems (macOS/Linux) with safe equivalents.
func sanitizeFilename(s string) string {
	// Replace slashes, colons and other problematic characters.
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", " -",
		"*", "",
		"?", "",
		"\"", "'",
		"<", "",
		">", "",
		"|", "-",
	)
	return strings.TrimSpace(replacer.Replace(s))
}

// fetchTMDBShow fetches (or returns cached) TMDB show data for the given show name.
// Returns nil if apiKey is empty or the show is not found.
func fetchTMDBShow(apiKey, showName string) *tmdbShowCache {
	if apiKey == "" {
		return nil
	}

	// Strip year suffix if present.
	queryName, _ := extractYearFromName(showName)

	cacheKey := strings.ToLower(queryName)

	tmdbCacheMu.RLock()
	cached, ok := tmdbCache[cacheKey]
	tmdbCacheMu.RUnlock()
	if ok {
		return cached
	}

	// Search TMDB.
	var searchResp tmdbSearchResponse
	if err := tmdbGet(apiKey, "/search/tv", url.Values{"query": {queryName}}, &searchResp); err != nil {
		logger.Warn("TMDB search failed", "show", showName, "error", err)
		return nil
	}
	if len(searchResp.Results) == 0 {
		logger.Warn("TMDB: no results", "show", showName)
		return nil
	}
	best := searchResp.Results[0]

	// Parse year from first air date.
	year := 0
	if len(best.FirstAirDate) >= 4 {
		fmt.Sscanf(best.FirstAirDate[:4], "%d", &year)
	}

	// Fetch external IDs for IMDb.
	var extIDs tmdbExternalIDs
	_ = tmdbGet(apiKey, fmt.Sprintf("/tv/%d/external_ids", best.ID), nil, &extIDs)

	entry := &tmdbShowCache{
		TMDBID:  best.ID,
		IMDBID:  extIDs.IMDbID,
		Year:    year,
		Seasons: make(map[int]map[int]string),
	}

	tmdbCacheMu.Lock()
	tmdbCache[cacheKey] = entry
	tmdbCacheMu.Unlock()

	return entry
}

// fetchTMDBSeason populates season episode names into the cache entry.
// Returns the map of episode number → name (may be empty on error).
func fetchTMDBSeason(apiKey string, entry *tmdbShowCache, seasonNum int) map[int]string {
	if entry == nil || apiKey == "" {
		return nil
	}

	tmdbCacheMu.RLock()
	eps, ok := entry.Seasons[seasonNum]
	tmdbCacheMu.RUnlock()
	if ok {
		return eps
	}

	var detail tmdbSeasonDetail
	if err := tmdbGet(apiKey, fmt.Sprintf("/tv/%d/season/%d", entry.TMDBID, seasonNum), nil, &detail); err != nil {
		logger.Warn("TMDB season detail failed", "id", entry.TMDBID, "season", seasonNum, "error", err)
		return nil
	}

	epMap := make(map[int]string, len(detail.Episodes))
	for _, e := range detail.Episodes {
		epMap[e.EpisodeNumber] = e.Name
	}

	tmdbCacheMu.Lock()
	entry.Seasons[seasonNum] = epMap
	tmdbCacheMu.Unlock()

	return epMap
}

// ClearTMDBCache clears the in-process TMDB show cache (e.g. on settings change).
func ClearTMDBCache() {
	tmdbCacheMu.Lock()
	tmdbCache = make(map[string]*tmdbShowCache)
	tmdbCacheMu.Unlock()
}
