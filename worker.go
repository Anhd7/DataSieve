package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// supportedExtensions defines which file types the scanner will process.
var supportedExtensions = map[string]bool{
	".csv":  true,
	".json": true,
	".txt":  true,
	".log":  true,
}

// WorkerPool orchestrates parallel file scanning using a fixed number of
// goroutines reading from a shared jobs channel.
//
// Architecture:
//
//	┌─────────────┐     jobs chan     ┌──────────┐
//	│  walkDir()  │ ──────────────► │ worker 1 │ ─┐
//	│  (producer) │                  ├──────────┤  │  results chan
//	└─────────────┘                  │ worker 2 │ ─┼──────────────► collector
//	                                 ├──────────┤  │
//	                                 │ worker N │ ─┘
//	                                 └──────────┘
type WorkerPool struct {
	numWorkers int
	jobs       chan string      // receives file paths to scan
	results    chan FileResult  // receives completed scan results
	wg         sync.WaitGroup  // tracks in-flight workers
}

// NewWorkerPool creates a pool with the specified concurrency level.
// numWorkers should typically match runtime.NumCPU() for I/O-bound workloads.
func NewWorkerPool(numWorkers int) *WorkerPool {
	return &WorkerPool{
		numWorkers: numWorkers,
		jobs:       make(chan string, numWorkers*4), // buffered to reduce blocking
		results:    make(chan FileResult, numWorkers*4),
	}
}

// Run starts all workers, walks rootDir to enqueue file paths, waits for
// completion, and returns the aggregated slice of FileResults.
func (wp *WorkerPool) Run(rootDir string) ([]FileResult, error) {
	// Launch workers — each goroutine blocks on the jobs channel until a path
	// arrives, scans the file, and sends the result downstream.
	for i := 0; i < wp.numWorkers; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}

	// Collect results concurrently so neither the workers nor the collector
	// block waiting for the other when channels fill up.
	var (
		allResults []FileResult
		mu         sync.Mutex
		collector  sync.WaitGroup
	)
	collector.Add(1)
	go func() {
		defer collector.Done()
		for result := range wp.results {
			mu.Lock()
			allResults = append(allResults, result)
			mu.Unlock()
		}
	}()

	// Walk the directory tree and push eligible file paths onto the jobs channel.
	// filepath.WalkDir is depth-first and single-threaded; the parallel work
	// happens inside the workers, not here.
	if err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err // propagate permission errors, etc.
		}
		if d.IsDir() {
			return nil // descend into subdirectories automatically
		}
		ext := strings.ToLower(filepath.Ext(path))
		if supportedExtensions[ext] {
			wp.jobs <- path // hand off to a free worker
		}
		return nil
	}); err != nil {
		// Close channels and drain to avoid goroutine leaks before returning.
		close(wp.jobs)
		wp.wg.Wait()
		close(wp.results)
		collector.Wait()
		return nil, err
	}

	// Signal workers that no more jobs are coming, then wait for them to finish.
	close(wp.jobs)
	wp.wg.Wait()

	// Once all workers are done, close the results channel so the collector exits.
	close(wp.results)
	collector.Wait()

	return allResults, nil
}

// worker is the goroutine function.  It reads file paths from wp.jobs until
// the channel is closed, scans each file, and writes the result to wp.results.
func (wp *WorkerPool) worker() {
	defer wp.wg.Done()
	for filePath := range wp.jobs {
		wp.results <- ScanFile(filePath)
	}
}