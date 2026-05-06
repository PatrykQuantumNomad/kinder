/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package snapshot

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// humanizeBytes formats a byte count into a human-readable string using
// binary (base-2) multipliers: TiB, GiB, MiB, KiB, B.
func humanizeBytes(n int64) string {
	const (
		tib = 1024 * 1024 * 1024 * 1024
		gib = 1024 * 1024 * 1024
		mib = 1024 * 1024
		kib = 1024
	)
	switch {
	case n >= tib:
		return fmt.Sprintf("%.1fTiB", float64(n)/tib)
	case n >= gib:
		return fmt.Sprintf("%.1fGiB", float64(n)/gib)
	case n >= mib:
		return fmt.Sprintf("%.1fMiB", float64(n)/mib)
	case n >= kib:
		return fmt.Sprintf("%.1fKiB", float64(n)/kib)
	default:
		return fmt.Sprintf("%dB", n)
	}
}

// humanizeAge returns a short human-readable string for a duration, e.g.
// "3h2m", "2d", "45s". Used for the AGE column in `kinder snapshot list`.
func humanizeAge(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	const (
		day  = 24 * time.Hour
		hour = time.Hour
		min  = time.Minute
	)
	switch {
	case d >= day:
		days := int(d / day)
		hours := int((d % day) / hour)
		if hours == 0 {
			return fmt.Sprintf("%dd", days)
		}
		return fmt.Sprintf("%dd%dh", days, hours)
	case d >= hour:
		hours := int(d / hour)
		mins := int((d % hour) / min)
		if mins == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh%dm", hours, mins)
	case d >= min:
		return fmt.Sprintf("%dm%ds", int(d/min), int((d%min)/time.Second))
	default:
		return fmt.Sprintf("%ds", int(d/time.Second))
	}
}

// formatAddons returns a sorted comma-separated "name=version" string for the
// addon map. Returns "" if the map is empty.
func formatAddons(addons map[string]string) string {
	if len(addons) == 0 {
		return ""
	}
	keys := sortedKeys(addons)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+addons[k])
	}
	return strings.Join(parts, ",")
}

// sortedKeys returns the keys of a map[string]string in sorted order.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// truncateString truncates s to at most maxRunes runes, appending "…" if
// truncation occurs. Used for the ADDONS column in `kinder snapshot list`.
func truncateString(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxRunes-1]) + "…"
}

// parseSize parses a human-readable size string into bytes. Supported suffixes
// (case-insensitive): T/TiB, G/GiB, M/MiB, K/KiB. No suffix = bytes.
// Uses binary (base-2) multipliers: 1K=1024, 1M=1024^2, etc.
// Returns 0, nil for an empty string (policy field not set).
func parseSize(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	s = strings.TrimSpace(s)
	upper := strings.ToUpper(s)

	// Strip "IB" suffix for TiB/GiB/MiB/KiB variants.
	trimmed := strings.TrimSuffix(upper, "IB")

	var suffix string
	var numStr string
	if len(trimmed) > 0 {
		last := trimmed[len(trimmed)-1]
		switch last {
		case 'T', 'G', 'M', 'K':
			suffix = string(last)
			numStr = trimmed[:len(trimmed)-1]
		default:
			// No suffix → trimmed is all digits.
			numStr = upper
		}
	} else {
		numStr = upper
	}

	var value float64
	if _, err := fmt.Sscanf(numStr, "%f", &value); err != nil || value < 0 {
		return 0, fmt.Errorf("invalid size %q: expected a positive number with optional suffix (K/M/G/T or KiB/MiB/GiB/TiB)", s)
	}

	const (
		kib = 1024
		mib = 1024 * 1024
		gib = 1024 * 1024 * 1024
		tib = 1024 * 1024 * 1024 * 1024
	)
	switch suffix {
	case "T":
		return int64(value * tib), nil
	case "G":
		return int64(value * gib), nil
	case "M":
		return int64(value * mib), nil
	case "K":
		return int64(value * kib), nil
	default:
		return int64(value), nil
	}
}
