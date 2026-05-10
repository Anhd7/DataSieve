// Package scanner provides PII detection patterns and matching logic
// aligned with the Saudi Personal Data Protection Law (PDPL).
package scanner

import "regexp"

// PIIType represents the category of personal data identified.
type PIIType string

const (
	PIIEmail      PIIType = "Email Address"
	PIIPhone      PIIType = "Saudi Phone Number"
	PIINationalID PIIType = "Saudi National ID"
)

// pattern holds a compiled regex and its associated PII type label.
type pattern struct {
	piiType PIIType
	regex   *regexp.Regexp
}

// patterns is the master list of PII detectors used by the scanner.
// All patterns are pre-compiled at startup for maximum throughput.
var patterns = []pattern{
	{
		// RFC 5322-inspired email pattern.
		piiType: PIIEmail,
		regex:   regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`),
	},
	{
		// Saudi mobile numbers: +966, 00966, or leading 0, followed by 5X XXXXXXXX.
		// Examples: +966512345678 | 00966 51 234 5678 | 0512345678
		piiType: PIIPhone,
		regex:   regexp.MustCompile(`(?:\+966|00966|0)\s?5\d[\s\-]?\d{3}[\s\-]?\d{4}`),
	},
	{
		// Saudi National ID: exactly 10 digits starting with 1 (Saudi citizen) or 2 (resident).
		// Negative lookaround equivalent: ensure not surrounded by other digits.
		piiType: PIINationalID,
		regex:   regexp.MustCompile(`(?:^|[^0-9])([12]\d{9})(?:[^0-9]|$)`),
	},
}

// Finding represents a single PII match within a file.
type Finding struct {
	PIIType PIIType // Category of PII detected
	Match   string  // The actual matched value (may be masked in output)
	Line    int     // 1-based line number in the source file
	Column  int     // 1-based column offset of the match start
}
