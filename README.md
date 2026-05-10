# DataScannerProject

A high-performance CLI tool that recursively scans directories for **Personal Identifiable Information (PII)** to support compliance with the **Saudi Personal Data Protection Law (PDPL)**.

---

## Features

| Capability | Detail |
|---|---|
| **PII Detection** | Emails, Saudi phone numbers (+966 / 00966 / 05x), Saudi National IDs (1xxxxxxxxx / 2xxxxxxxxx) |
| **File Types** | `.csv`, `.json`, `.txt`, `.log` |
| **Parallel Scanning** | Worker Pool pattern — goroutines + channels, configurable concurrency |
| **Value Masking** | Matched PII is partially redacted in output (e.g. `ah*****om`) |
| **CI/CD Integration** | Exits with code `2` when PII is found; `0` when clean |
| **Zero Dependencies** | Uses only Go's standard library (`regexp`, `os`, `path/filepath`, `bufio`) |

---

## Architecture

```
main.go
  └── WorkerPool.Run(rootDir)
        ├── filepath.WalkDir()  ──► jobs channel  (buffered)
        │
        ├── worker goroutine 1  ◄── jobs channel
        ├── worker goroutine 2       │
        └── worker goroutine N       └──► ScanFile(path) ──► results channel
                                                               │
                                                         collector goroutine
                                                               │
                                                         []FileResult
```

**Why this pattern?**  
`filepath.WalkDir` is single-threaded by design. The worker pool decouples I/O-bound file reading from directory traversal: the walker enqueues paths into a buffered `jobs` channel while `N` goroutines consume and scan files concurrently. A separate collector goroutine drains the `results` channel so neither producers nor consumers block.

---

## Project Structure

```
DataScannerProject/
├── main.go                   # CLI entry point, flag parsing
├── go.mod                    # Module: DataScannerProject
├── README.md
├── scanner/
│   ├── patterns.go           # PII regex patterns & Finding type
│   ├── scanner.go            # ScanFile() — line-by-line regex matching
│   ├── worker.go             # WorkerPool — goroutines & channels
│   └── scanner_test.go       # Unit + integration tests
├── reporter/
│   └── reporter.go           # ANSI colour output & summary stats
└── testdata/
    ├── employees.csv         # Sample file with PII
    ├── customers.json        # Sample file with PII
    ├── app.log               # Log file with embedded PII
    ├── catalogue.txt         # Clean file (no PII)
    └── records/
        └── nested/
            └── contacts.csv  # Nested directory test
```

---

## Getting Started

**Prerequisites:** Go 1.21+

```bash
# Clone and enter the project
git clone https://github.com/your-org/DataScannerProject
cd DataScannerProject

# Initialize the module (first time only)
go mod init DataScannerProject

# Build
go build -o DataScannerProject .

# Scan a directory with default settings (workers = number of CPUs)
./DataScannerProject -dir ./data

# Scan with explicit concurrency
./DataScannerProject -dir ./data -workers 8

# Print version
./DataScannerProject -version
```

---

## Module Imports

The module name is `DataScannerProject`. Internal imports across the project follow this convention:

| File | Import |
|---|---|
| `main.go` | `"DataScannerProject/scanner"`, `"DataScannerProject/reporter"` |
| `reporter/reporter.go` | `"DataScannerProject/scanner"` |

---

## PII Patterns

### Email Address
```regexp
[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}
```
Matches standard email addresses per RFC 5322 conventions.

### Saudi Phone Number
```regexp
(?:\+966|00966|0)\s?5\d[\s\-]?\d{3}[\s\-]?\d{4}
```
Matches Saudi mobile numbers in all common formats:
- `+966512345678`
- `00966 55 987 6543`
- `0501234567`

### Saudi National ID
```regexp
(?:^|[^0-9])([12]\d{9})(?:[^0-9]|$)
```
Matches the 10-digit National ID starting with `1` (Saudi citizen) or `2` (resident). Boundary anchors prevent false positives from longer numeric strings.

---

## Running Tests

```bash
go test ./scanner/... -v
```

Tests cover:
- Individual regex pattern validation (valid + invalid cases)
- `ScanFile` — email, phone, national ID detection; clean files; missing files; multi-line coordinates
- `WorkerPool` — recursive directory walk, extension filtering, concurrent result collection

---

## Exit Codes

| Code | Meaning |
|---|---|
| `0` | Scan completed — no PII found |
| `1` | Fatal error (bad directory, permission denied, etc.) |
| `2` | Scan completed — **PII detected**, review required |

Exit code `2` allows this tool to gate CI/CD pipelines:

```yaml
# Example GitHub Actions step
- name: PII Scan
  run: ./DataScannerProject -dir ./data
  # Step fails automatically if PII is present (exit 2 != 0)
```

---

## PDPL Compliance Note

This tool assists in identifying personal data as defined under Saudi Arabia's **Personal Data Protection Law (PDPL)**. Detection is heuristic-based (regex); results should be reviewed by a qualified Data Protection Officer before formal compliance reporting.