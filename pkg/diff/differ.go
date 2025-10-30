// Package diff provides utilities for comparing markdown strings.
// It depends only on standard library.
package diff

import (
	"fmt"
	"strings"
)

// Edit represents a single edit operation in the diff
type Edit struct {
	Type int    // 0=equal, 1=delete, 2=insert
	Line string
}

const (
	EditEqual  = 0
	EditDelete = 1
	EditInsert = 2
)

// Diff computes a simple diff between two markdown strings
func Diff(before, after string) string {
	beforeLines := strings.Split(before, "\n")
	afterLines := strings.Split(after, "\n")

	var result []string

	// Simple line-by-line diff
	maxLen := len(beforeLines)
	if len(afterLines) > maxLen {
		maxLen = len(afterLines)
	}

	for i := 0; i < maxLen; i++ {
		var beforeLine, afterLine string
		if i < len(beforeLines) {
			beforeLine = beforeLines[i]
		}
		if i < len(afterLines) {
			afterLine = afterLines[i]
		}

		if beforeLine == afterLine {
			// Unchanged
			if beforeLine != "" {
				result = append(result, "  "+beforeLine)
			}
		} else {
			// Changed
			if beforeLine != "" {
				result = append(result, "- "+beforeLine)
			}
			if afterLine != "" {
				result = append(result, "+ "+afterLine)
			}
		}
	}

	return strings.Join(result, "\n")
}

// CompactDiff returns only changed lines (removes unchanged context)
func CompactDiff(before, after string) string {
	beforeLines := strings.Split(before, "\n")
	afterLines := strings.Split(after, "\n")

	var result []string
	changed := false

	maxLen := len(beforeLines)
	if len(afterLines) > maxLen {
		maxLen = len(afterLines)
	}

	for i := 0; i < maxLen; i++ {
		var beforeLine, afterLine string
		if i < len(beforeLines) {
			beforeLine = strings.TrimSpace(beforeLines[i])
		}
		if i < len(afterLines) {
			afterLine = strings.TrimSpace(afterLines[i])
		}

		if beforeLine != afterLine {
			changed = true
			if beforeLine != "" {
				result = append(result, fmt.Sprintf("- %s", beforeLines[i]))
			}
			if afterLine != "" {
				result = append(result, fmt.Sprintf("+ %s", afterLines[i]))
			}
		}
	}

	if !changed {
		return "(No changes detected)"
	}

	return strings.Join(result, "\n")
}

// RenderDiff renders diff in unified format
func RenderDiff(before, after string) string {
	return UnifiedDiff(before, after, 3)
}

// UnifiedDiff creates a unified diff with context lines
func UnifiedDiff(before, after string, contextLines int) string {
	beforeLines := strings.Split(before, "\n")
	afterLines := strings.Split(after, "\n")

	// Compute LCS-based diff
	edits := computeDiff(beforeLines, afterLines)

	if len(edits) == 0 {
		return "(No changes detected)"
	}

	// Group edits into hunks
	hunks := groupIntoHunks(edits, contextLines)

	if len(hunks) == 0 {
		return "(No changes detected)"
	}

	// Render hunks
	var result []string
	for _, hunk := range hunks {
		result = append(result, renderHunk(hunk, beforeLines, afterLines)...)
	}

	return strings.Join(result, "\n")
}

// Hunk represents a contiguous block of changes
type Hunk struct {
	BeforeStart int
	BeforeCount int
	AfterStart  int
	AfterCount  int
	Edits       []Edit
}

// computeDiff computes the diff using Myers' algorithm (simplified)
func computeDiff(before, after []string) []Edit {
	n := len(before)
	m := len(after)

	// Build LCS table
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}

	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if before[i-1] == after[j-1] {
				lcs[i][j] = lcs[i-1][j-1] + 1
			} else {
				if lcs[i-1][j] > lcs[i][j-1] {
					lcs[i][j] = lcs[i-1][j]
				} else {
					lcs[i][j] = lcs[i][j-1]
				}
			}
		}
	}

	// Backtrack to build edits
	var edits []Edit
	i, j := n, m
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && before[i-1] == after[j-1] {
			edits = append([]Edit{{Type: EditEqual, Line: before[i-1]}}, edits...)
			i--
			j--
		} else if j > 0 && (i == 0 || lcs[i][j-1] >= lcs[i-1][j]) {
			edits = append([]Edit{{Type: EditInsert, Line: after[j-1]}}, edits...)
			j--
		} else if i > 0 {
			edits = append([]Edit{{Type: EditDelete, Line: before[i-1]}}, edits...)
			i--
		}
	}

	return edits
}

// groupIntoHunks groups edits into hunks with context
func groupIntoHunks(edits []Edit, contextLines int) []Hunk {
	var hunks []Hunk
	var currentHunk *Hunk
	beforeIdx := 0
	afterIdx := 0
	unchangedCount := 0

	for i, edit := range edits {
		if edit.Type == EditEqual {
			unchangedCount++
			// If we have a current hunk and too many unchanged lines, close it
			if currentHunk != nil && unchangedCount > contextLines*2 {
				// Add trailing context
				for j := 0; j < contextLines && i-unchangedCount+j < len(edits); j++ {
					if edits[i-unchangedCount+j].Type == EditEqual {
						currentHunk.Edits = append(currentHunk.Edits, edits[i-unchangedCount+j])
						currentHunk.BeforeCount++
						currentHunk.AfterCount++
					}
				}
				hunks = append(hunks, *currentHunk)
				currentHunk = nil
				unchangedCount = 0
			}
			beforeIdx++
			afterIdx++
		} else {
			// Start new hunk if needed
			if currentHunk == nil {
				currentHunk = &Hunk{
					BeforeStart: beforeIdx,
					AfterStart:  afterIdx,
				}
				// Add leading context
				start := i - unchangedCount
				if start < 0 {
					start = 0
				}
				contextStart := start
				if unchangedCount > contextLines {
					contextStart = i - contextLines
				}
				for j := contextStart; j < i; j++ {
					if edits[j].Type == EditEqual {
						currentHunk.Edits = append(currentHunk.Edits, edits[j])
						currentHunk.BeforeCount++
						currentHunk.AfterCount++
						currentHunk.BeforeStart--
						currentHunk.AfterStart--
					}
				}
			}
			unchangedCount = 0

			currentHunk.Edits = append(currentHunk.Edits, edit)
			if edit.Type == EditDelete {
				currentHunk.BeforeCount++
				beforeIdx++
			} else if edit.Type == EditInsert {
				currentHunk.AfterCount++
				afterIdx++
			}
		}
	}

	// Close last hunk
	if currentHunk != nil {
		hunks = append(hunks, *currentHunk)
	}

	return hunks
}

// renderHunk renders a single hunk in unified format
func renderHunk(hunk Hunk, beforeLines, afterLines []string) []string {
	var result []string

	// Hunk header
	header := fmt.Sprintf("@@ -%d,%d +%d,%d @@",
		hunk.BeforeStart+1, hunk.BeforeCount,
		hunk.AfterStart+1, hunk.AfterCount)
	result = append(result, header)

	// Render edits
	for _, edit := range hunk.Edits {
		switch edit.Type {
		case EditEqual:
			result = append(result, " "+edit.Line)
		case EditDelete:
			result = append(result, "-"+edit.Line)
		case EditInsert:
			result = append(result, "+"+edit.Line)
		}
	}

	return result
}
