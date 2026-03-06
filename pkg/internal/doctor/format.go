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

// categoryGroup holds results for a single category.
type categoryGroup struct {
	Name    string
	Results []Result
}

// FormatHumanReadable renders results grouped by category with Unicode icons.
// Output goes to the provided writer. Category order preserves first-seen
// order from the input slice (matching AllChecks() registry order).
func FormatHumanReadable(w io.Writer, results []Result) {
	categories := groupByCategory(results)

	for _, cat := range categories {
		fmt.Fprintf(w, "\n=== %s ===\n", cat.Name)
		for _, r := range cat.Results {
			switch r.Status {
			case "ok":
				fmt.Fprintf(w, "  \u2713 %s\n", r.Message)
			case "fail":
				fmt.Fprintf(w, "  \u2717 %s\n", r.Name)
				fmt.Fprintf(w, "    %s\n", r.Reason)
				fmt.Fprintf(w, "    \u2192 %s\n", r.Fix)
			case "warn":
				fmt.Fprintf(w, "  \u26A0 %s\n", r.Name)
				fmt.Fprintf(w, "    %s\n", r.Reason)
				fmt.Fprintf(w, "    \u2192 %s\n", r.Fix)
			case "skip":
				fmt.Fprintf(w, "  \u2298 %s %s\n", r.Name, r.Message)
			}
		}
	}

	// Summary separator and count
	fmt.Fprintf(w, "\n\u2500\u2500\u2500\n")
	ok, warn, fail, skip := countStatuses(results)
	total := ok + warn + fail + skip
	fmt.Fprintf(w, "%d checks: %d ok, %d warning, %d failed, %d skipped\n",
		total, ok, warn, fail, skip)
}

// FormatJSON returns an envelope map suitable for JSON serialization.
// The envelope contains a "checks" array and a "summary" object with counts.
func FormatJSON(results []Result) map[string]interface{} {
	ok, warn, fail, skip := countStatuses(results)
	total := ok + warn + fail + skip

	return map[string]interface{}{
		"checks": results,
		"summary": map[string]int{
			"total": total,
			"ok":    ok,
			"warn":  warn,
			"fail":  fail,
			"skip":  skip,
		},
	}
}

// groupByCategory groups results by category preserving first-seen order.
func groupByCategory(results []Result) []categoryGroup {
	seen := make(map[string]int) // category name -> index in groups
	var groups []categoryGroup

	for _, r := range results {
		idx, ok := seen[r.Category]
		if !ok {
			idx = len(groups)
			seen[r.Category] = idx
			groups = append(groups, categoryGroup{Name: r.Category})
		}
		groups[idx].Results = append(groups[idx].Results, r)
	}

	return groups
}

// countStatuses counts the number of results in each status.
func countStatuses(results []Result) (ok, warn, fail, skip int) {
	for _, r := range results {
		switch r.Status {
		case "ok":
			ok++
		case "warn":
			warn++
		case "fail":
			fail++
		case "skip":
			skip++
		}
	}
	return
}
