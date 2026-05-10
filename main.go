// Governata PII Scanner — main entry point.
//
// Usage:
//
//	governata-scanner [flags]
//
// Flags:
//
//	-dir      string   Directory to scan (default: current working directory)
//	-workers  int      Number of parallel workers (default: 4)
//	-version          Print version and exit
//
// The scanner walks the target directory recursively, processes every .csv,
// .json, .txt, and .log file using a configurable worker pool, and reports
// any PII found — masking matched values to prevent accidental exposure.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	"DataScannerProject/reporter"
	"DataScannerProject/scanner"
)

const version = "1.0.0"

func main() {
	// ── CLI flags ──────────────────────────────────────────────────────────────
	dir := flag.String("dir", ".", "Directory to scan for PII")
	workers := flag.Int("workers", runtime.NumCPU(), "Number of parallel scan workers")
	showVersion := flag.Bool("version", false, "Print version information and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Governata PII Scanner — Saudi PDPL compliance tool\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s -dir ./data\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -dir ./data -workers 8\n", os.Args[0])
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("governata-scanner v%s (go%s %s/%s)\n",
			version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	// ── Validate target directory ──────────────────────────────────────────────
	info, err := os.Stat(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot access directory %q: %v\n", *dir, err)
		os.Exit(1)
	}
	if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: %q is not a directory\n", *dir)
		os.Exit(1)
	}

	if *workers < 1 {
		fmt.Fprintf(os.Stderr, "Error: -workers must be at least 1\n")
		os.Exit(1)
	}

	// ── Run the scan ───────────────────────────────────────────────────────────
	reporter.PrintBanner()

	pool := scanner.NewWorkerPool(*workers)

	start := time.Now()
	results, err := pool.Run(*dir)
	elapsed := time.Since(start)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Scan error: %v\n", err)
		os.Exit(1)
	}

	reporter.PrintResults(results, *dir, *workers)
	fmt.Printf("  Completed in %s\n\n", elapsed.Round(time.Millisecond))

	// Exit with code 2 when PII is found — useful for CI/CD pipeline integration.
	for _, r := range results {
		if len(r.Findings) > 0 {
			os.Exit(2)
		}
	}
}