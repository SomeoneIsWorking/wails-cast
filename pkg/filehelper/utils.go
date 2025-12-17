package filehelper

import (
	"regexp"
	"strings"
)

// Define characters that are illegal or problematic on most major operating systems (Windows, Unix).
// This includes control characters and filesystem-specific separators/wildcards.
// The regex pattern matches: < > : " / \ | ? *
var illegalChars = regexp.MustCompile(`[<>:"/\\|?*]`)

// reservedNames are Windows system names that can cause issues if used as a filename (without extension).
// Pre-calculating the map improves performance over using a switch or slice search.
var reservedNames = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true,
	"COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true,
	"LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// ConvertToUsableFilename cleans an arbitrary string to create a safe filename.
func ConvertToUsableFilename(s string) string {
	// 1. Trim leading/trailing whitespace
	cleaned := strings.TrimSpace(s)

	// 2. Replace illegal characters with a safe substitute (underscore)
	cleaned = illegalChars.ReplaceAllString(cleaned, "_")

	// 3. Handle edge case of an empty string after cleaning
	if cleaned == "" {
		return "default_file"
	}

	// 4. Truncate long filenames to prevent OS path limit issues (e.g., max 255 bytes)
	const maxLength = 200
	if len(cleaned) > maxLength {
		cleaned = cleaned[:maxLength]
	}

	// 5. Check for and neutralize Windows reserved filenames (case-insensitive)
	baseName := cleaned

	// Separate base name from extension, if present
	if dotIndex := strings.LastIndex(cleaned, "."); dotIndex != -1 {
		baseName = cleaned[:dotIndex]
	}

	// If the base name (uppercase) matches a reserved name, prepend an underscore
	if reservedNames[strings.ToUpper(baseName)] {
		// Only prepend if it's an exact match of a reserved word, not just a file that STARTS with it.
		cleaned = "_" + cleaned
	}

	return cleaned
}
