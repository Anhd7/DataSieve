// Package reporter handles all terminal output for the Governata PII Scanner.
// It uses ANSI escape codes directly (no external dependencies) to produce
// colour-coded, human-readable reports.
package reporter

import (
	"fmt"
	"sort"
	"strings"

	"DataScannerProject/scanner"
)

// ANSI colour codes — fall back gracefully on terminals that don't support them.
const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[97m"
	colorGray   = "\033[90m"
)

// piiColor returns a distinct colour per PII category for quick visual triage.
func piiColor(t scanner.PIIType) string {
	switch t {
	case scanner.PIIEmail:
		return colorCyan
	case scanner.PIIPhone:
		return colorYellow
	case scanner.PIINationalID:
		return colorRed
	default:
		return colorWhite
	}
}

// maskValue redacts the middle portion of a matched value so sensitive data
// is never printed verbatim in reports or logs.
func maskValue(s string) string {
	r := []rune(s)
	n := len(r)
	if n <= 4 {
		return strings.Repeat("*", n)
	}
	// Keep first 2 and last 2 characters; mask everything in between.
	return string(r[:2]) + strings.Repeat("*", n-4) + string(r[n-2:])
}

// PrintBanner prints the tool header.
func PrintBanner() {
	fmt.Println()
	fmt.Printf("%s%s╔══════════════════════════════════════════════╗%s\n", colorBold, colorCyan, colorReset)
	fmt.Printf("%s%s║   GOVERNATA — PDPL PII Scanner  v1.0.0      ║%s\n", colorBold, colorCyan, colorReset)
	fmt.Printf("%s%s║   Saudi Personal Data Protection Law        ║%s\n", colorBold, colorCyan, colorReset)
	fmt.Printf("%s%s╚══════════════════════════════════════════════╝%s\n", colorBold, colorCyan, colorReset)
	fmt.Println()
}

// PrintResults outputs the per-file findings, then a consolidated summary.
func PrintResults(results []scanner.FileResult, rootDir string, numWorkers int) {
	// Sort results alphabetically by file path for deterministic output.
	sort.Slice(results, func(i, j int) bool {
		return results[i].FilePath < results[j].FilePath
	})

	totalFiles := len(results)
	filesWithPII := 0
	errorFiles := 0

	// Counters per PII category for the summary table.
	counts := map[scanner.PIIType]int{
		scanner.PIIEmail:      0,
		scanner.PIIPhone:      0,
		scanner.PIINationalID: 0,
	}

	// ── Per-file section ─────────────────────────────────────────────────────
	fmt.Printf("%s%sScan Results — Directory: %s%s\n", colorBold, colorWhite, rootDir, colorReset)
	fmt.Printf("%sWorkers: %d%s\n\n", colorGray, numWorkers, colorReset)

	for _, r := range results {
		if r.Err != nil {
			errorFiles++
			fmt.Printf("%s  [ERROR]%s %s — %v\n", colorRed, colorReset, r.FilePath, r.Err)
			continue
		}

		if len(r.Findings) == 0 {
			fmt.Printf("%s  [CLEAN]%s %s\n", colorGreen, colorReset, r.FilePath)
			continue
		}

		filesWithPII++
		fmt.Printf("%s%s  [PII FOUND]%s %s\n", colorBold, colorRed, colorReset, r.FilePath)

		for _, f := range r.Findings {
			counts[f.PIIType]++
			color := piiColor(f.PIIType)
			fmt.Printf(
				"    %s│%s %sLine %-4d Col %-3d%s  %-22s  %s%s%s\n",
				colorGray, colorReset,
				colorGray, f.Line, f.Column, colorReset,
				fmt.Sprintf("[%s]", f.PIIType),
				color, maskValue(f.Match), colorReset,
			)
		}
		fmt.Println()
	}

	// ── Summary table ─────────────────────────────────────────────────────────
	totalFindings := counts[scanner.PIIEmail] + counts[scanner.PIIPhone] + counts[scanner.PIINationalID]

	fmt.Printf("%s%s────────────────────────────────────────────────%s\n", colorBold, colorGray, colorReset)
	fmt.Printf("%s%s  SCAN SUMMARY%s\n", colorBold, colorWhite, colorReset)
	fmt.Printf("%s%s────────────────────────────────────────────────%s\n", colorBold, colorGray, colorReset)
	fmt.Printf("  Files scanned      : %s%d%s\n", colorBold, totalFiles, colorReset)
	fmt.Printf("  Files with PII     : %s%d%s\n", colorRed, filesWithPII, colorReset)
	fmt.Printf("  Files clean        : %s%d%s\n", colorGreen, totalFiles-filesWithPII-errorFiles, colorReset)
	fmt.Printf("  Files with errors  : %s%d%s\n", colorYellow, errorFiles, colorReset)
	fmt.Println()
	fmt.Printf("  %-24s %s%d%s\n", string(scanner.PIIEmail)+":", colorCyan, counts[scanner.PIIEmail], colorReset)
	fmt.Printf("  %-24s %s%d%s\n", string(scanner.PIIPhone)+":", colorYellow, counts[scanner.PIIPhone], colorReset)
	fmt.Printf("  %-24s %s%d%s\n", string(scanner.PIINationalID)+":", colorRed, counts[scanner.PIINationalID], colorReset)
	fmt.Printf("%s%s────────────────────────────────────────────────%s\n", colorBold, colorGray, colorReset)

	if totalFindings == 0 {
		fmt.Printf("\n%s%s  ✓ No PII detected. Directory appears compliant.%s\n\n", colorBold, colorGreen, colorReset)
	} else {
		fmt.Printf("\n%s%s  ⚠ Total PII findings: %d — review required under PDPL.%s\n\n",
			colorBold, colorRed, totalFindings, colorReset)
	}
}