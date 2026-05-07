/*
Copyright 2019 The Kubernetes Authors.

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

package doctor

import (
	"fmt"
	"io"
)

// ---------------------------------------------------------------------------
// Internal JSON serialization structs (unexported — keeps JSON keys off the
// engine types, which don't need them for internal use).
// ---------------------------------------------------------------------------

type decodeMatchJSON struct {
	PatternID   string `json:"pattern_id"`
	Scope       string `json:"scope"`
	Explanation string `json:"explanation"`
	Fix         string `json:"fix"`
	DocLink     string `json:"doc_link"`
	Source      string `json:"source"`
	Line        string `json:"line"`
}

type decodeSummaryJSON struct {
	TotalMatches int            `json:"total_matches"`
	TotalLines   int            `json:"total_lines"`
	ByScope      map[string]int `json:"by_scope"`
}

// ---------------------------------------------------------------------------
// scopeGroup holds matches under a single scope heading for human rendering.
// ---------------------------------------------------------------------------

type scopeGroup struct {
	Scope   string
	Matches []DecodeMatch
}

// groupByScope groups matches preserving first-seen scope order (mirrors
// groupByCategory in format.go).
func groupByScope(matches []DecodeMatch) []scopeGroup {
	seen := make(map[string]int) // scope -> index in groups
	var groups []scopeGroup
	for _, m := range matches {
		scope := string(m.Pattern.Scope)
		idx, ok := seen[scope]
		if !ok {
			idx = len(groups)
			seen[scope] = idx
			groups = append(groups, scopeGroup{Scope: scope})
		}
		groups[idx].Matches = append(groups[idx].Matches, m)
	}
	return groups
}

// ---------------------------------------------------------------------------
// FormatDecodeHumanReadable renders a DecodeResult in grouped, human-readable
// format to w.  Uses fmt.Fprintf and Unicode icons to mirror format.go's style.
// Does NOT import lifecycle (renderer is engine-tier).
// ---------------------------------------------------------------------------

// FormatDecodeHumanReadable writes a grouped human-readable representation of
// result to w.  Each scope gets a heading; each match shows pattern ID,
// explanation, fix, and doc link (when present).  Empty-result cases print a
// friendly message.  A summary line at the end shows total lines scanned and
// match count.
func FormatDecodeHumanReadable(w io.Writer, result *DecodeResult) {
	totalLines := len(result.Matches) + result.Unmatched

	// Header.
	if result.Cluster != "" {
		fmt.Fprintf(w, "=== Decode Results: %s ===\n", result.Cluster)
	} else {
		fmt.Fprintf(w, "=== Decode Results ===\n")
	}

	// Empty-result cases.
	if len(result.Matches) == 0 {
		if totalLines == 0 {
			fmt.Fprintf(w, "\nNo logs or events to scan.\n")
		} else {
			fmt.Fprintf(w, "\nNo known patterns matched (scanned %d lines).\n", totalLines)
		}
		fmt.Fprintf(w, "\n───\n")
		fmt.Fprintf(w, "%d lines scanned, 0 patterns matched.\n", totalLines)
		return
	}

	// Grouped output.
	groups := groupByScope(result.Matches)
	for _, g := range groups {
		fmt.Fprintf(w, "\n--- %s ---\n", g.Scope)
		for _, m := range g.Matches {
			fmt.Fprintf(w, "  [%s] %s\n", m.Pattern.ID, m.Pattern.Explanation)
			fmt.Fprintf(w, "    Source: %s\n", m.Source)
			fmt.Fprintf(w, "    Line:   %s\n", m.Line)
			fmt.Fprintf(w, "    Fix:    %s\n", m.Pattern.Fix)
			if m.Pattern.DocLink != "" {
				fmt.Fprintf(w, "    Docs:   %s\n", m.Pattern.DocLink)
			}
		}
	}

	// Summary separator and counts.
	fmt.Fprintf(w, "\n───\n")
	fmt.Fprintf(w, "%d lines scanned, %d pattern(s) matched.\n", totalLines, len(result.Matches))
}

// ---------------------------------------------------------------------------
// FormatDecodeJSON returns a map[string]interface{} envelope suitable for
// encoding with json.NewEncoder.  All SC3 fields are preserved on each match
// entry via unexported decodeMatchJSON structs (DIAG-03 compliance).
// ---------------------------------------------------------------------------

// FormatDecodeJSON builds the JSON envelope for a DecodeResult.  The caller
// encodes it, e.g.: json.NewEncoder(streams.Out).Encode(FormatDecodeJSON(r)).
func FormatDecodeJSON(result *DecodeResult) map[string]interface{} {
	totalLines := len(result.Matches) + result.Unmatched

	matchesOut := make([]decodeMatchJSON, 0, len(result.Matches))
	byScope := make(map[string]int)
	for _, m := range result.Matches {
		matchesOut = append(matchesOut, decodeMatchJSON{
			PatternID:   m.Pattern.ID,
			Scope:       string(m.Pattern.Scope),
			Explanation: m.Pattern.Explanation,
			Fix:         m.Pattern.Fix,
			DocLink:     m.Pattern.DocLink,
			Source:      m.Source,
			Line:        m.Line,
		})
		byScope[string(m.Pattern.Scope)]++
	}

	return map[string]interface{}{
		"cluster":   result.Cluster,
		"matches":   matchesOut,
		"unmatched": result.Unmatched,
		"summary": decodeSummaryJSON{
			TotalMatches: len(result.Matches),
			TotalLines:   totalLines,
			ByScope:      byScope,
		},
	}
}
