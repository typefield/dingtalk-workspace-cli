package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type DateRotatingWriter struct {
	mu        sync.Mutex
	dir       string
	curDate   string
	file      *os.File
	retention int
}

func NewDateRotatingWriter(dir string, retentionDays int) (*DateRotatingWriter, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("audit: create dir: %w", err)
	}
	w := &DateRotatingWriter{
		dir:       dir,
		retention: retentionDays,
	}
	go w.pruneOldFiles()
	return w, nil
}

func (w *DateRotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	today := time.Now().Format("20060102")
	if today != w.curDate || w.file == nil {
		if w.file != nil {
			w.file.Close()
		}
		path := filepath.Join(w.dir, fmt.Sprintf("audit-%s.jsonl", today))
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return 0, fmt.Errorf("audit: open file: %w", err)
		}
		w.file = f
		w.curDate = today
	}

	return w.file.Write(p)
}

func (w *DateRotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

func (w *DateRotatingWriter) pruneOldFiles() {
	if w.retention <= 0 {
		return
	}
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -w.retention).Format("20060102")
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "audit-") || !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		dateStr := strings.TrimPrefix(name, "audit-")
		dateStr = strings.TrimSuffix(dateStr, ".jsonl")
		if len(dateStr) != 8 {
			continue
		}
		if dateStr < cutoff {
			_ = os.Remove(filepath.Join(w.dir, name))
		}
	}
}

func (w *DateRotatingWriter) Dir() string {
	return w.dir
}

func LatestAuditFile(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var files []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "audit-") && strings.HasSuffix(e.Name(), ".jsonl") {
			files = append(files, e.Name())
		}
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no audit files found in %s", dir)
	}
	sort.Strings(files)
	return filepath.Join(dir, files[len(files)-1]), nil
}

func AuditFilesInRange(dir, since, until string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "audit-") || !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		dateStr := strings.TrimPrefix(name, "audit-")
		dateStr = strings.TrimSuffix(dateStr, ".jsonl")
		if len(dateStr) != 8 {
			continue
		}
		if (since == "" || dateStr >= since) && (until == "" || dateStr <= until) {
			files = append(files, filepath.Join(dir, name))
		}
	}
	sort.Strings(files)
	return files, nil
}
