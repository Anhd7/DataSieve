package scanner

import (
	"bufio"
	"os"
	"strings"
)

// FileResult contains all PII findings discovered in a single file.
type FileResult struct {
	FilePath string    // Absolute or relative path to the scanned file
	Findings []Finding // All matches found; empty if the file is clean
	Err      error     // Non-nil if the file could not be read
}

// ScanFile opens the given file, reads it line by line, and applies every
// registered PII pattern against each line. It returns a FileResult that
// aggregates all matches together with their source coordinates.
func ScanFile(filePath string) FileResult {
	result := FileResult{FilePath: filePath}

	f, err := os.Open(filePath)
	if err != nil {
		result.Err = err
		return result
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		for _, p := range patterns {
			matches := p.regex.FindAllStringIndex(line, -1)
			for _, loc := range matches {
				start, end := loc[0], loc[1]
				matched := strings.TrimSpace(line[start:end])

				// For the National ID pattern the full match may include a
				// surrounding non-digit boundary character. Strip it so we
				// surface only the 10-digit ID itself.
				if p.piiType == PIINationalID {
					// The capture group is sub-match [1]; extract it directly.
					sub := p.regex.FindStringSubmatch(line[start:end])
					if len(sub) > 1 {
						matched = sub[1]
						// Adjust column to the digit start, not the boundary char.
						if len(line[start:end]) > len(matched) {
							start++
						}
					}
				}

				result.Findings = append(result.Findings, Finding{
					PIIType: p.piiType,
					Match:   matched,
					Line:    lineNum,
					Column:  start + 1, // convert to 1-based
				})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		result.Err = err
	}

	return result
}
