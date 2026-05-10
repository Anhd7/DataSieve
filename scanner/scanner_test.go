package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

// ── Pattern tests ──────────────────────────────────────────────────────────────

func TestEmailPattern(t *testing.T) {
	valid := []string{
		"user@example.com",
		"ahmed.rashidi@aramco.com.sa",
		"user+tag@sub.domain.org",
		"m.ghamdi@stc.com.sa",
	}
	invalid := []string{
		"not-an-email",
		"missing@",
		"@nodomain.com",
	}
	p := patterns[0] // email pattern
	for _, v := range valid {
		if !p.regex.MatchString(v) {
			t.Errorf("email pattern should match %q", v)
		}
	}
	for _, v := range invalid {
		if p.regex.MatchString(v) {
			t.Errorf("email pattern should NOT match %q", v)
		}
	}
}

func TestSaudiPhonePattern(t *testing.T) {
	valid := []string{
		"+966512345678",
		"00966512345678",
		"0512345678",
		"+966 55 678 9012",
		"00966 51 234 5678",
	}
	invalid := []string{
		"123456789",    // no country code
		"+1 800 555 0100", // US number
	}
	p := patterns[1] // phone pattern
	for _, v := range valid {
		if !p.regex.MatchString(v) {
			t.Errorf("phone pattern should match %q", v)
		}
	}
	for _, v := range invalid {
		if p.regex.MatchString(v) {
			t.Errorf("phone pattern should NOT match %q", v)
		}
	}
}

func TestNationalIDPattern(t *testing.T) {
	valid := []string{
		" 1098765432 ",  // citizen (starts with 1)
		" 2076543219 ",  // resident (starts with 2)
	}
	invalid := []string{
		" 3012345678 ", // starts with 3 — invalid
		" 123456789 ",  // only 9 digits
		" 10987654321 ", // 11 digits — too long
	}
	p := patterns[2] // national ID pattern
	for _, v := range valid {
		if !p.regex.MatchString(v) {
			t.Errorf("national ID pattern should match %q", v)
		}
	}
	for _, v := range invalid {
		if p.regex.MatchString(v) {
			t.Errorf("national ID pattern should NOT match %q", v)
		}
	}
}

// ── ScanFile tests ─────────────────────────────────────────────────────────────

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	f.Close()
	return f.Name()
}

func TestScanFile_FindsEmail(t *testing.T) {
	path := writeTempFile(t, "Contact us at admin@governata.sa for support.\n")
	result := ScanFile(path)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if len(result.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(result.Findings))
	}
	if result.Findings[0].PIIType != PIIEmail {
		t.Errorf("expected PIIEmail, got %q", result.Findings[0].PIIType)
	}
	if result.Findings[0].Line != 1 {
		t.Errorf("expected line 1, got %d", result.Findings[0].Line)
	}
}

func TestScanFile_FindsPhone(t *testing.T) {
	path := writeTempFile(t, "Call us at +966512345678 anytime.\n")
	result := ScanFile(path)
	if len(result.Findings) != 1 || result.Findings[0].PIIType != PIIPhone {
		t.Errorf("expected exactly 1 phone finding")
	}
}

func TestScanFile_FindsNationalID(t *testing.T) {
	path := writeTempFile(t, "Verify ID: 1098765432 in system.\n")
	result := ScanFile(path)
	hasNID := false
	for _, f := range result.Findings {
		if f.PIIType == PIINationalID {
			hasNID = true
			break
		}
	}
	if !hasNID {
		t.Error("expected a National ID finding")
	}
}

func TestScanFile_CleanFile(t *testing.T) {
	path := writeTempFile(t, "SKU: PROD-001 | Price: SAR 99 | In stock: yes\n")
	result := ScanFile(path)
	if len(result.Findings) != 0 {
		t.Errorf("expected no findings in clean file, got %d", len(result.Findings))
	}
}

func TestScanFile_NonExistentFile(t *testing.T) {
	result := ScanFile("/does/not/exist.txt")
	if result.Err == nil {
		t.Error("expected an error for missing file")
	}
}

func TestScanFile_MultipleLines(t *testing.T) {
	content := "line 1: nothing here\nline 2: email user@test.com phone +966501234567\nline 3: clean\n"
	path := writeTempFile(t, content)
	result := ScanFile(path)

	linesSeen := map[int]bool{}
	for _, f := range result.Findings {
		linesSeen[f.Line] = true
	}
	if !linesSeen[2] {
		t.Error("expected findings on line 2")
	}
	if linesSeen[1] || linesSeen[3] {
		t.Error("did not expect findings on lines 1 or 3")
	}
}

// ── WorkerPool integration test ────────────────────────────────────────────────

func TestWorkerPool_RecursiveScan(t *testing.T) {
	// Build a small temp directory tree:
	//   root/
	//     a.txt  — has email
	//     sub/
	//       b.csv  — has phone
	//       c.md   — unsupported extension, should be ignored
	root := t.TempDir()

	os.WriteFile(filepath.Join(root, "a.txt"), []byte("admin@example.com\n"), 0644)
	os.MkdirAll(filepath.Join(root, "sub"), 0755)
	os.WriteFile(filepath.Join(root, "sub", "b.csv"), []byte("name,phone\nAli,+966512345678\n"), 0644)
	os.WriteFile(filepath.Join(root, "sub", "c.md"), []byte("# ignore this file\n"), 0644)

	pool := NewWorkerPool(2)
	results, err := pool.Run(root)
	if err != nil {
		t.Fatalf("pool.Run returned error: %v", err)
	}

	// c.md should NOT appear in results (unsupported extension).
	for _, r := range results {
		if filepath.Ext(r.FilePath) == ".md" {
			t.Errorf("unsupported .md file was scanned: %s", r.FilePath)
		}
	}

	// We should have scanned exactly 2 files.
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	// Total findings: 1 email + 1 phone = 2
	total := 0
	for _, r := range results {
		total += len(r.Findings)
	}
	if total != 2 {
		t.Errorf("expected 2 total findings, got %d", total)
	}
}
