package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Logger is the interface for logging from watchers.
type Logger interface {
	Log(source, message string)
}

// FileLogger writes to log files and optionally prints to console.
type FileLogger struct {
	logsDir string
	console bool
	files   map[string]*os.File
	mu      sync.Mutex
}

// NewFileLogger creates a file logger.
func NewFileLogger(logsDir string, console bool) (*FileLogger, error) {
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, fmt.Errorf("create logs dir: %w", err)
	}
	return &FileLogger{
		logsDir: logsDir,
		console: console,
		files:   make(map[string]*os.File),
	}, nil
}

// Log implements Logger - writes to file and optionally console.
func (l *FileLogger) Log(source, message string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	clean := stripColorTags(message)
	line := fmt.Sprintf("%s [%s] %s\n", timestamp, source, clean)

	// Get or create file for this source
	file, err := l.getFile(source)
	if err == nil {
		file.WriteString(line)
	}

	// Also write to main log
	if source != "main" {
		mainFile, err := l.getFile("main")
		if err == nil {
			mainFile.WriteString(line)
		}
	}

	// Console output if enabled
	if l.console {
		fmt.Print(line)
	}
}

func (l *FileLogger) getFile(source string) (*os.File, error) {
	if f, ok := l.files[source]; ok {
		return f, nil
	}

	filename := source + ".log"
	path := filepath.Join(l.logsDir, filename)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	l.files[source] = f
	return f, nil
}

// Close closes all open log files.
func (l *FileLogger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, f := range l.files {
		f.Close()
	}
}

// stripColorTags removes [color] and [-] tview formatting
func stripColorTags(s string) string {
	result := ""
	i := 0
	for i < len(s) {
		if s[i] == '[' && i+1 < len(s) {
			// Check if this is a color tag
			j := i + 1
			for j < len(s) && s[j] != ']' && s[j] != '[' {
				j++
			}
			if j < len(s) && s[j] == ']' {
				// Skip the tag
				i = j + 1
				continue
			}
		}
		result += string(s[i])
		i++
	}
	return result
}
