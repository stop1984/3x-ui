// Package version compares 3x-ui release versions ("v3.8.0" or "3.8.0").
package version

import (
	"regexp"
	"strconv"
	"strings"
)

// Compare returns -1, 0 or 1 as a is older than, equal to or newer than b, and
// false when either side is not MAJOR.MINOR.PATCH with an optional fork revision.
func Compare(a, b string) (int, bool) {
	aParts, okA := parse(a)
	bParts, okB := parse(b)
	if !okA || !okB {
		return 0, false
	}
	for i := range len(aParts) {
		if aParts[i] > bParts[i] {
			return 1, true
		}
		if aParts[i] < bParts[i] {
			return -1, true
		}
	}
	return 0, true
}

// Normalize strips surrounding space and a leading "v" from a version tag.
func Normalize(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

var releasePattern = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:-stop1984\.(\d+))?$`)

func parse(v string) ([4]int, bool) {
	var result [4]int
	parts := releasePattern.FindStringSubmatch(Normalize(v))
	if parts == nil {
		return result, false
	}
	for i, part := range parts[1:] {
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return result, false
		}
		result[i] = n
	}
	return result, true
}
