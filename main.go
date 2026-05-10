package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"DataScannerProject/scanner"
)

type ScanResponse struct {
	Results  []scanner.FileResult `json:"results"`
	Workers  int                  `json:"workers"`
	Duration string               `json:"duration"`
	Error    string               `json:"error,omitempty"`
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", serveIndex)
	http.HandleFunc("/api/scan", serveScan)

	log.Printf("Governata PII Scanner — listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, indexHTML)
}

// serveScan accepts multipart uploads, writes them to a temp dir,
// runs the scanner, then cleans up.
func serveScan(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// 32 MB max upload
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		json.NewEncoder(w).Encode(ScanResponse{Error: "failed to parse upload: " + err.Error()})
		return
	}

	tmpDir, err := os.MkdirTemp("", "governata-scan-*")
	if err != nil {
		json.NewEncoder(w).Encode(ScanResponse{Error: "could not create temp dir: " + err.Error()})
		return
	}
	defer os.RemoveAll(tmpDir)

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		json.NewEncoder(w).Encode(ScanResponse{Error: "no files uploaded"})
		return
	}

	for _, fh := range files {
		src, err := fh.Open()
		if err != nil {
			continue
		}
		dst, err := os.Create(filepath.Join(tmpDir, filepath.Base(fh.Filename)))
		if err != nil {
			src.Close()
			continue
		}
		io.Copy(dst, src)
		src.Close()
		dst.Close()
	}

	workers := runtime.NumCPU()
	pool := scanner.NewWorkerPool(workers)

	start := time.Now()
	results, err := pool.Run(tmpDir)
	elapsed := time.Since(start)

	resp := ScanResponse{
		Workers:  workers,
		Duration: elapsed.Round(time.Millisecond).String(),
	}
	if err != nil {
		resp.Error = err.Error()
	} else {
		resp.Results = results
	}

	json.NewEncoder(w).Encode(resp)
}

const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1.0"/>
<title>Governata · PII Scanner</title>
<link href="https://fonts.googleapis.com/css2?family=Bebas+Neue&family=IBM+Plex+Mono:wght@400;600&family=IBM+Plex+Sans:wght@400;500;600&display=swap" rel="stylesheet"/>
<style>
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

:root {
  --ink:      #0a0a0a;
  --paper:    #f2efe8;
  --mid:      #d6d1c8;
  --muted:    #8a8478;
  --alert:    #c8001e;
  --safe:     #006644;
  --warn-bg:  #fff0f0;
  --safe-bg:  #f0fff8;
  --mono:     'IBM Plex Mono', monospace;
  --sans:     'IBM Plex Sans', sans-serif;
  --display:  'Bebas Neue', sans-serif;
}

html, body {
  background: var(--paper);
  color: var(--ink);
  font-family: var(--sans);
  min-height: 100vh;
}

body::before {
  content:'';
  position:fixed; inset:0;
  background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 512 512' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.75' numOctaves='4' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)' opacity='1'/%3E%3C/svg%3E");
  opacity:.04;
  pointer-events:none;
  z-index:0;
}

.page {
  position: relative; z-index: 1;
  max-width: 900px;
  margin: 0 auto;
  padding: 0 24px 80px;
}

.masthead {
  border-bottom: 3px solid var(--ink);
  padding: 28px 0 16px;
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 40px;
}

.title {
  font-family: var(--display);
  font-size: clamp(52px, 8vw, 88px);
  line-height: 0.9;
  letter-spacing: 0.02em;
  color: var(--ink);
}
.title .accent { color: var(--alert); }

.meta {
  text-align: right;
  font-family: var(--mono);
  font-size: 10px;
  color: var(--muted);
  line-height: 1.8;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  flex-shrink: 0;
}

.rule {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 28px;
}
.rule-label {
  font-family: var(--mono);
  font-size: 10px;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: var(--muted);
  white-space: nowrap;
}
.rule-line { flex: 1; height: 1px; background: var(--mid); }

.drop-zone {
  border: 2px dashed var(--mid);
  padding: 48px 24px;
  text-align: center;
  cursor: pointer;
  transition: border-color 0.2s, background 0.2s;
  margin-bottom: 20px;
  position: relative;
}
.drop-zone:hover, .drop-zone.drag-over {
  border-color: var(--ink);
  background: rgba(0,0,0,0.02);
}
.drop-zone input[type=file] {
  position: absolute; inset: 0;
  opacity: 0; cursor: pointer; width: 100%; height: 100%;
}
.drop-icon { font-size: 32px; margin-bottom: 12px; line-height: 1; }
.drop-title {
  font-family: var(--mono);
  font-size: 13px;
  font-weight: 600;
  letter-spacing: 0.06em;
  color: var(--ink);
  margin-bottom: 6px;
}
.drop-sub {
  font-family: var(--mono);
  font-size: 11px;
  color: var(--muted);
  letter-spacing: 0.06em;
}

#file-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 20px;
}

.chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 10px;
  background: var(--ink);
  color: var(--paper);
  font-family: var(--mono);
  font-size: 11px;
  letter-spacing: 0.04em;
  animation: chipIn 0.2s ease both;
}
@keyframes chipIn {
  from { opacity: 0; transform: scale(0.9); }
  to   { opacity: 1; transform: scale(1); }
}
.chip-remove {
  cursor: pointer; opacity: 0.5; font-size: 13px;
  line-height: 1; background: none; border: none;
  color: inherit; padding: 0;
}
.chip-remove:hover { opacity: 1; }

.scan-btn {
  width: 100%;
  padding: 16px;
  background: var(--ink);
  color: var(--paper);
  font-family: var(--display);
  font-size: 22px;
  letter-spacing: 0.14em;
  border: none;
  cursor: pointer;
  transition: background 0.15s;
  margin-bottom: 40px;
}
.scan-btn:hover:not(:disabled) { background: var(--alert); }
.scan-btn:disabled { opacity: 0.4; cursor: not-allowed; }

#progress { display: none; height: 3px; background: var(--mid); margin-bottom: 40px; overflow: hidden; }
#progress.active { display: block; }
.progress-bar {
  height: 100%;
  background: var(--ink);
  animation: progAnim 1.4s ease-in-out infinite;
}
@keyframes progAnim {
  0%   { width: 0%;  margin-left: 0%; }
  50%  { width: 60%; margin-left: 20%; }
  100% { width: 0%;  margin-left: 100%; }
}

.stats-row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  border: 1px solid var(--ink);
  margin-bottom: 40px;
  opacity: 0;
  transition: opacity 0.4s;
}
.stats-row.visible { opacity: 1; }
.stat { padding: 20px 16px; border-right: 1px solid var(--ink); }
.stat:last-child { border-right: none; }
.stat-label {
  font-family: var(--mono);
  font-size: 9px;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--muted);
  margin-bottom: 8px;
}
.stat-value { font-family: var(--display); font-size: 40px; line-height: 1; color: var(--ink); }
.stat-value.alert { color: var(--alert); }
.stat-value.safe  { color: var(--safe); }
.stat-dur { font-family: var(--mono); font-size: 18px; font-weight: 600; color: var(--ink); padding-top: 6px; }

#results { display: none; }
#results.visible { display: block; }

.verdict {
  padding: 16px 20px;
  margin-bottom: 28px;
  font-family: var(--mono);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  border-left: 4px solid;
}
.verdict.danger { background: var(--warn-bg); border-color: var(--alert); color: var(--alert); }
.verdict.ok     { background: var(--safe-bg); border-color: var(--safe);  color: var(--safe); }

.file-block {
  border: 1px solid var(--mid);
  margin-bottom: 10px;
  animation: slideIn 0.25s ease both;
}
@keyframes slideIn {
  from { opacity: 0; transform: translateY(6px); }
  to   { opacity: 1; transform: translateY(0); }
}

.file-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  cursor: pointer;
  user-select: none;
  gap: 12px;
}
.file-header:hover { background: rgba(0,0,0,0.03); }
.file-name {
  font-family: var(--mono);
  font-size: 12px;
  font-weight: 600;
  color: var(--ink);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.file-badge {
  font-family: var(--mono);
  font-size: 10px;
  font-weight: 600;
  padding: 3px 9px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  flex-shrink: 0;
}
.file-badge.pii   { background: var(--alert); color: #fff; }
.file-badge.clean { background: var(--safe);  color: #fff; }

.findings { border-top: 1px solid var(--mid); overflow: hidden; }
.findings.collapsed { display: none; }

table { width: 100%; border-collapse: collapse; font-family: var(--mono); font-size: 11.5px; }
th {
  text-align: left; padding: 8px 16px;
  font-size: 9px; letter-spacing: 0.16em; text-transform: uppercase;
  color: var(--muted); background: rgba(0,0,0,0.03); border-bottom: 1px solid var(--mid);
}
td { padding: 9px 16px; border-bottom: 1px solid var(--mid); vertical-align: middle; }
tr:last-child td { border-bottom: none; }

.type-badge {
  display: inline-block; padding: 2px 7px;
  font-size: 9px; font-weight: 600; letter-spacing: 0.1em;
  text-transform: uppercase; border: 1px solid;
}
.type-Email      { border-color: #0055cc; color: #0055cc; }
.type-Phone      { border-color: #996600; color: #996600; }
.type-NationalID,
.type-National   { border-color: #7700aa; color: #7700aa; }

.masked { color: var(--alert); font-weight: 600; }
.ln { color: var(--muted); }

.error-box {
  border: 1px solid var(--alert); background: var(--warn-bg);
  padding: 16px; font-family: var(--mono); font-size: 12px;
  color: var(--alert); display: none; margin-bottom: 24px;
}
.error-box.visible { display: block; }

footer {
  border-top: 1px solid var(--mid);
  padding-top: 20px; margin-top: 60px;
  font-family: var(--mono); font-size: 10px;
  color: var(--muted); letter-spacing: 0.08em;
  display: flex; justify-content: space-between;
}

@media (max-width: 600px) {
  .stats-row { grid-template-columns: repeat(2, 1fr); }
  .stat:nth-child(2) { border-right: none; }
  .stat-value { font-size: 32px; }
  .masthead { flex-direction: column; align-items: flex-start; }
  .meta { text-align: left; }
}
</style>
</head>
<body>
<div class="page">

  <div class="masthead">
    <div class="title">GOVERN<span class="accent">ATA</span></div>
    <div class="meta">
      PII SCANNER · V1.0.0<br>
      SAUDI PDPL COMPLIANCE<br>
      GO · STANDARD LIBRARY
    </div>
  </div>

  <div class="rule">
    <span class="rule-label">Upload Files</span>
    <span class="rule-line"></span>
  </div>

  <div class="drop-zone" id="dropZone">
    <input type="file" id="fileInput" multiple accept=".csv,.json,.txt,.log"/>
    <div class="drop-icon">⬆</div>
    <div class="drop-title">Drop files here or click to browse</div>
    <div class="drop-sub">Accepts .csv · .json · .txt · .log</div>
  </div>

  <div id="file-chips"></div>

  <button class="scan-btn" id="scanBtn" onclick="runScan()" disabled>
    RUN SCAN
  </button>

  <div id="progress"><div class="progress-bar"></div></div>

  <div class="stats-row" id="stats">
    <div class="stat">
      <div class="stat-label">Files</div>
      <div class="stat-value" id="s-files">—</div>
    </div>
    <div class="stat">
      <div class="stat-label">Findings</div>
      <div class="stat-value" id="s-findings">—</div>
    </div>
    <div class="stat">
      <div class="stat-label">Affected</div>
      <div class="stat-value" id="s-affected">—</div>
    </div>
    <div class="stat">
      <div class="stat-label">Duration</div>
      <div class="stat-dur" id="s-duration">—</div>
    </div>
  </div>

  <div class="error-box" id="error-box"></div>

  <div id="results">
    <div class="rule" style="margin-bottom:20px">
      <span class="rule-label" id="results-label">Results</span>
      <span class="rule-line"></span>
    </div>
    <div class="verdict" id="verdict"></div>
    <div id="file-list"></div>
  </div>

  <footer>
    <span>GOVERNATA PII SCANNER</span>
    <span>SAUDI PDPL COMPLIANCE TOOL</span>
  </footer>

</div>
<script>
let selectedFiles = [];

const dropZone  = document.getElementById('dropZone');
const fileInput = document.getElementById('fileInput');

dropZone.addEventListener('dragover', e => { e.preventDefault(); dropZone.classList.add('drag-over'); });
dropZone.addEventListener('dragleave', () => dropZone.classList.remove('drag-over'));
dropZone.addEventListener('drop', e => {
  e.preventDefault();
  dropZone.classList.remove('drag-over');
  addFiles(Array.from(e.dataTransfer.files));
});
fileInput.addEventListener('change', () => {
  addFiles(Array.from(fileInput.files));
  fileInput.value = '';
});

function addFiles(files) {
  const allowed = ['.csv','.json','.txt','.log'];
  files.forEach(f => {
    const ext = '.' + f.name.split('.').pop().toLowerCase();
    if (!allowed.includes(ext)) return;
    if (!selectedFiles.find(x => x.name === f.name)) selectedFiles.push(f);
  });
  renderChips();
}

function removeFile(name) {
  selectedFiles = selectedFiles.filter(f => f.name !== name);
  renderChips();
}

function renderChips() {
  const c = document.getElementById('file-chips');
  c.innerHTML = '';
  selectedFiles.forEach(f => {
    const chip = document.createElement('div');
    chip.className = 'chip';
    chip.innerHTML = esc(f.name) + '<button class="chip-remove" onclick="removeFile(\'' + esc(f.name).replace(/\\/g,'\\\\').replace(/'/g,"\\'") + '\')">✕</button>';
    c.appendChild(chip);
  });
  document.getElementById('scanBtn').disabled = selectedFiles.length === 0;
}

async function runScan() {
  if (!selectedFiles.length) return;
  const btn = document.getElementById('scanBtn');
  btn.disabled = true;
  btn.textContent = 'SCANNING…';
  document.getElementById('progress').classList.add('active');
  document.getElementById('results').classList.remove('visible');
  document.getElementById('stats').classList.remove('visible');
  document.getElementById('error-box').classList.remove('visible');

  const form = new FormData();
  selectedFiles.forEach(f => form.append('files', f));

  try {
    const res  = await fetch('/api/scan', { method: 'POST', body: form });
    const data = await res.json();
    document.getElementById('progress').classList.remove('active');
    if (data.error) { showError(data.error); return; }
    renderStats(data);
    renderResults(data.results || []);
  } catch(e) {
    document.getElementById('progress').classList.remove('active');
    showError('Request failed: ' + e.message);
  } finally {
    btn.disabled = false;
    btn.textContent = 'RUN SCAN AGAIN';
  }
}

function showError(msg) {
  const eb = document.getElementById('error-box');
  eb.textContent = '✕ ' + msg;
  eb.classList.add('visible');
}

function renderStats(data) {
  const results  = data.results || [];
  const findings = results.reduce((s, r) => s + (r.Findings || []).length, 0);
  const affected = results.filter(r => (r.Findings||[]).length > 0).length;

  document.getElementById('s-files').textContent    = results.length;
  document.getElementById('s-duration').textContent = data.duration || '—';

  const sf = document.getElementById('s-findings');
  sf.textContent = findings;
  sf.className = 'stat-value ' + (findings > 0 ? 'alert' : 'safe');

  const sa = document.getElementById('s-affected');
  sa.textContent = affected;
  sa.className = 'stat-value ' + (affected > 0 ? 'alert' : 'safe');

  document.getElementById('stats').classList.add('visible');
}

function renderResults(results) {
  const list = document.getElementById('file-list');
  list.innerHTML = '';
  const total = results.reduce((s, r) => s + (r.Findings||[]).length, 0);

  const verdict = document.getElementById('verdict');
  if (total > 0) {
    verdict.className = 'verdict danger';
    verdict.textContent = '⚠ PII detected across ' + results.filter(r=>(r.Findings||[]).length>0).length + ' file(s) — review required before sharing';
  } else {
    verdict.className = 'verdict ok';
    verdict.textContent = '✓ No PII found — all ' + results.length + ' file(s) are clean';
  }

  [...results]
    .sort((a,b) => (b.Findings||[]).length - (a.Findings||[]).length)
    .forEach((file, i) => {
      const findings = file.Findings || [];
      const hasPII   = findings.length > 0;
      const name     = (file.Path || file.path || 'unknown').split(/[\\/]/).pop();

      const block = document.createElement('div');
      block.className = 'file-block';
      block.style.animationDelay = (i * 0.04) + 's';

      block.innerHTML =
        '<div class="file-header" onclick="toggle(this)">' +
          '<span class="file-name">' + esc(name) + '</span>' +
          '<span class="file-badge ' + (hasPII ? 'pii' : 'clean') + '">' +
            (hasPII ? findings.length + ' finding' + (findings.length > 1 ? 's' : '') : 'Clean') +
          '</span>' +
        '</div>';

      if (hasPII) {
        const rows = findings.map(f => {
          const type = f.Type  || f.type  || '';
          const val  = f.Value || f.value || '';
          const line = f.Line  || f.line  || '—';
          const key  = type.replace(/\s+/g,'');
          return '<tr>' +
            '<td class="ln">' + esc(String(line)) + '</td>' +
            '<td><span class="type-badge type-' + key + '">' + esc(type) + '</span></td>' +
            '<td class="masked">' + esc(val) + '</td>' +
          '</tr>';
        }).join('');

        block.innerHTML +=
          '<div class="findings">' +
            '<table><thead><tr><th>Line</th><th>Type</th><th>Masked Value</th></tr></thead>' +
            '<tbody>' + rows + '</tbody></table>' +
          '</div>';
      }

      list.appendChild(block);
    });

  document.getElementById('results').classList.add('visible');
}

function toggle(header) {
  const body = header.nextElementSibling;
  if (body) body.classList.toggle('collapsed');
}

function esc(s) {
  return String(s)
    .replace(/&/g,'&amp;').replace(/</g,'&lt;')
    .replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}
</script>
</body>
</html>`
