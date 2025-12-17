package subtitles

import (
	"fmt"
	"strconv"
	"strings"
)

// SubtitleEntry represents a single subtitle entry with timing information
type SubtitleEntry struct {
	Text     string  `json:"text"`
	Duration float64 `json:"duration"`
	Delay    float64 `json:"delay"`
}

// WebVTTJson represents a collection of subtitle entries
type WebVTTJson struct {
	Entries []SubtitleEntry `json:"entries"`
}

// ToSimpleFormat converts WebVTTJson to simple text format
func (w *WebVTTJson) ToSimpleFormat() string {
	var sb strings.Builder

	for _, entry := range w.Entries {
		fmt.Fprintf(&sb, "delay: %.3f\n", entry.Delay)
		fmt.Fprintf(&sb, "duration: %.3f\n", entry.Duration)
		fmt.Fprintf(&sb, "%s\n\n", entry.Text)
	}

	return sb.String()
}

// ParseSimpleFormat parses simple text format and returns WebVTTJson
func ParseSimpleFormat(content string) (*WebVTTJson, error) {
	var entries []SubtitleEntry
	lines := strings.Split(content, "\n")

	for i := 0; i < len(lines); {
		line := strings.TrimSpace(lines[i])

		if line == "" {
			i++
			continue
		}

		// Parse delay line
		if !strings.HasPrefix(line, "delay:") {
			i++
			continue
		}
		delayStr := strings.TrimSpace(strings.TrimPrefix(line, "delay:"))
		delay, err := strconv.ParseFloat(delayStr, 64)
		if err != nil {
			delay = 0.0
		}

		// Parse duration line
		i++
		if i >= len(lines) {
			break
		}
		durationLine := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(durationLine, "duration:") {
			continue
		}
		durationStr := strings.TrimSpace(strings.TrimPrefix(durationLine, "duration:"))
		duration, err := strconv.ParseFloat(durationStr, 64)
		if err != nil {
			duration = 2.0
		}

		// Parse text line
		i++
		if i >= len(lines) {
			break
		}
		text := strings.TrimSpace(lines[i])

		entries = append(entries, SubtitleEntry{
			Text:     text,
			Duration: duration,
			Delay:    delay,
		})

		i++
	}

	return &WebVTTJson{Entries: entries}, nil
}

// Parse parses WebVTT content and converts to WebVTTJson format
func Parse(vttContent string) (*WebVTTJson, error) {
	var rawEntries []rawEntry

	// Normalize line endings
	vttContent = strings.ReplaceAll(vttContent, "\r\n", "\n")

	// Split by double newlines to get individual subtitle blocks
	blocks := strings.SplitSeq(vttContent, "\n\n")

	for block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" || strings.HasPrefix(block, "WEBVTT") || strings.HasPrefix(block, "NOTE") {
			continue
		}

		lines := strings.Split(block, "\n")
		if len(lines) < 2 {
			continue
		}

		// Find the timestamp line (contains -->)
		var timestampLine string
		var textLines []string

		for _, line := range lines {
			if strings.Contains(line, "-->") {
				timestampLine = line
			} else if timestampLine != "" && strings.TrimSpace(line) != "" {
				// Skip sequence numbers (lines that are just numbers)
				if _, err := strconv.Atoi(strings.TrimSpace(line)); err == nil && timestampLine == "" {
					continue
				}
				// This is subtitle text
				textLines = append(textLines, line)
			}
		}

		if timestampLine == "" {
			continue
		}

		// Normalize SRT-style timestamps (comma) to VTT-style (dot)
		timestampLine = strings.ReplaceAll(timestampLine, ",", ".")

		// Parse timestamps
		parts := strings.Split(timestampLine, "-->")
		if len(parts) != 2 {
			continue
		}

		startTime, err := parseTimestamp(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, fmt.Errorf("invalid start timestamp in block: %s - %w", timestampLine, err)
		}

		endTime, err := parseTimestamp(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, fmt.Errorf("invalid end timestamp in block: %s - %w", timestampLine, err)
		}

		text := strings.TrimSpace(strings.Join(textLines, "\n"))

		if text == "" {
			continue
		}

		duration := endTime - startTime
		if duration <= 0 {
			return nil, fmt.Errorf("invalid duration (end <= start) in block: %s", timestampLine)
		}

		rawEntries = append(rawEntries, rawEntry{
			startTime: startTime,
			endTime:   endTime,
			text:      text,
		})
	}

	// Now split overlapping entries
	entries := splitOverlappingEntries(rawEntries)

	return &WebVTTJson{Entries: entries}, nil
}

type rawEntry struct {
	startTime float64
	endTime   float64
	text      string
}

// splitOverlappingEntries takes raw subtitle entries and splits them to handle overlaps
func splitOverlappingEntries(rawEntries []rawEntry) []SubtitleEntry {
	if len(rawEntries) == 0 {
		return nil
	}

	// Collect all time boundaries
	type timePoint struct {
		time  float64
		texts []string // Active texts at this segment
	}

	// Get all unique timestamps
	timeSet := make(map[float64]bool)
	for _, entry := range rawEntries {
		timeSet[entry.startTime] = true
		timeSet[entry.endTime] = true
	}

	// Convert to sorted slice
	var times []float64
	for t := range timeSet {
		times = append(times, t)
	}

	// Sort times
	for i := 0; i < len(times); i++ {
		for j := i + 1; j < len(times); j++ {
			if times[j] < times[i] {
				times[i], times[j] = times[j], times[i]
			}
		}
	}

	// Build segments
	var entries []SubtitleEntry
	var currentTime float64

	for i := 0; i < len(times)-1; i++ {
		segmentStart := times[i]
		segmentEnd := times[i+1]

		// Find all texts active in this segment
		var activeTexts []string
		for _, entry := range rawEntries {
			if entry.startTime <= segmentStart && entry.endTime >= segmentEnd {
				activeTexts = append(activeTexts, entry.text)
			}
		}

		if len(activeTexts) > 0 {
			combinedText := strings.Join(activeTexts, "\n")
			duration := segmentEnd - segmentStart
			delay := segmentStart - currentTime

			entries = append(entries, SubtitleEntry{
				Text:     combinedText,
				Duration: duration,
				Delay:    delay,
			})

			currentTime = segmentEnd
		}
	}

	return entries
}

// ToWebVTTString converts WebVTTJson to WebVTT format string
func (w *WebVTTJson) ToWebVTTString() string {
	var vtt strings.Builder
	vtt.WriteString("WEBVTT\n\n")

	var currentTime float64

	for i, entry := range w.Entries {
		startTime := currentTime + entry.Delay
		endTime := startTime + entry.Duration

		startTimestamp := formatTimestamp(startTime)
		endTimestamp := formatTimestamp(endTime)

		vtt.WriteString(fmt.Sprintf("%d\n", i+1))
		vtt.WriteString(fmt.Sprintf("%s --> %s\n", startTimestamp, endTimestamp))
		vtt.WriteString(fmt.Sprintf("%s\n\n", entry.Text))

		currentTime = endTime
	}

	return vtt.String()
}

// RemoveClosedCaptions returns a stripped clone where closed captions are removed
func (w *WebVTTJson) RemoveClosedCaptions() *WebVTTJson {
	if w == nil {
		return nil
	}

	// First compute absolute start times for original entries so we can
	// preserve timing when removing entries.
	starts := make([]float64, len(w.Entries))
	current := 0.0
	for i, e := range w.Entries {
		start := current + e.Delay
		starts[i] = start
		current = start + e.Duration
	}

	// Build cloned entries, dropping entries that are only CC after stripping.
	var out []SubtitleEntry
	var prevEnd float64
	for i, e := range w.Entries {
		t := e.Text

		// Remove bracketed sequences like [text]
		for {
			startIdx := strings.Index(t, "[")
			endIdx := strings.Index(t, "]")
			if startIdx == -1 || endIdx == -1 || endIdx < startIdx {
				break
			}
			t = t[:startIdx] + t[endIdx+1:]
		}

		// Collapse whitespace and trim
		t = strings.TrimSpace(strings.Join(strings.Fields(t), " "))

		// If text is empty after stripping, or if it only contains dashes
		// and whitespace (e.g. "-" or "--"), skip this entry entirely.
		if t == "" {
			continue
		}
		tmp := strings.ReplaceAll(t, "-", "")
		tmp = strings.TrimSpace(tmp)
		if tmp == "" {
			continue
		}

		// Compute delay relative to previous kept entry
		start := starts[i]
		delay := start - prevEnd
		if len(out) == 0 {
			// For the first kept entry, delay should be its absolute start
			delay = start
		}

		out = append(out, SubtitleEntry{
			Text:     t,
			Duration: e.Duration,
			Delay:    delay,
		})

		prevEnd = start + e.Duration
	}

	return &WebVTTJson{Entries: out}
}

// parseTimestamp converts HH:MM:SS.mmm or MM:SS.mmm to seconds
func parseTimestamp(timestamp string) (float64, error) {
	parts := strings.Split(timestamp, ":")

	var hours, minutes, seconds int
	var millis int
	var err error

	if len(parts) == 3 {
		// HH:MM:SS.mmm format
		hours, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, fmt.Errorf("invalid hours: %w", err)
		}
		minutes, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, fmt.Errorf("invalid minutes: %w", err)
		}
		secParts := strings.Split(parts[2], ".")
		seconds, err = strconv.Atoi(secParts[0])
		if err != nil {
			return 0, fmt.Errorf("invalid seconds: %w", err)
		}
		if len(secParts) > 1 {
			millis, err = strconv.Atoi(secParts[1])
			if err != nil {
				return 0, fmt.Errorf("invalid milliseconds: %w", err)
			}
		}
	} else if len(parts) == 2 {
		// MM:SS.mmm format
		minutes, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, fmt.Errorf("invalid minutes: %w", err)
		}
		secParts := strings.Split(parts[1], ".")
		seconds, err = strconv.Atoi(secParts[0])
		if err != nil {
			return 0, fmt.Errorf("invalid seconds: %w", err)
		}
		if len(secParts) > 1 {
			millis, err = strconv.Atoi(secParts[1])
			if err != nil {
				return 0, fmt.Errorf("invalid milliseconds: %w", err)
			}
		}
	} else {
		return 0, fmt.Errorf("invalid timestamp format: %s (expected HH:MM:SS.mmm or MM:SS.mmm)", timestamp)
	}

	return float64(hours*3600+minutes*60+seconds) + float64(millis)/1000.0, nil
}

// formatTimestamp converts seconds to WebVTT timestamp format (HH:MM:SS.mmm)
func formatTimestamp(seconds float64) string {
	hours := int(seconds) / 3600
	minutes := (int(seconds) % 3600) / 60
	secs := int(seconds) % 60
	millis := int((seconds - float64(int(seconds))) * 1000)
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, secs, millis)
}
