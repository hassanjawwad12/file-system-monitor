package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileEvent represents a file system event with timestamp
type FileEvent struct {
	Path      string
	Operation string
	Timestamp time.Time
}

// Monitor handles the file system monitoring operations
type Monitor struct {
	watcher    *fsnotify.Watcher
	events     chan FileEvent
	done       chan bool
	eventLog   []FileEvent
	eventMutex sync.RWMutex
}

// NewMonitor creates a new file system monitor
func NewMonitor() (*Monitor, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %v", err)
	}

	return &Monitor{
		watcher:  watcher,
		events:   make(chan FileEvent),
		done:     make(chan bool),
		eventLog: make([]FileEvent, 0),
	}, nil
}

// Start begins monitoring the specified directory
func (m *Monitor) Start(path string) error {
	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %v", err)
	}

	// Verify directory exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("directory does not exist: %s", absPath)
	}

	// Add directory to watcher
	if err := m.watcher.Add(absPath); err != nil {
		return fmt.Errorf("failed to add directory to watcher: %v", err)
	}

	// Start event processing goroutine
	go m.processEvents()

	fmt.Printf("Started monitoring directory: %s\n", absPath)
	return nil
}

// processEvents handles the file system events
func (m *Monitor) processEvents() {
	for {
		select {
		case event, ok := <-m.watcher.Events:
			if !ok {
				return
			}
			fileEvent := FileEvent{
				Path:      event.Name,
				Operation: event.Op.String(),
				Timestamp: time.Now(),
			}
			m.logEvent(fileEvent)
			m.events <- fileEvent

		case err, ok := <-m.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Error: %v\n", err)

		case <-m.done:
			return
		}
	}
}

// logEvent adds an event to the event log
func (m *Monitor) logEvent(event FileEvent) {
	m.eventMutex.Lock()
	defer m.eventMutex.Unlock()
	m.eventLog = append(m.eventLog, event)
}

// GetEventLog returns a copy of the event log
func (m *Monitor) GetEventLog() []FileEvent {
	m.eventMutex.RLock()
	defer m.eventMutex.RUnlock()
	logCopy := make([]FileEvent, len(m.eventLog))
	copy(logCopy, m.eventLog)
	return logCopy
}

// Stop stops the file system monitor
func (m *Monitor) Stop() {
	m.done <- true
	m.watcher.Close()
}

func main() {
	// Create new monitor
	monitor, err := NewMonitor()
	if err != nil {
		log.Fatalf("Failed to create monitor: %v", err)
	}

	// Get directory to monitor from command line args or use current directory
	dir := "."
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	// Start monitoring
	if err := monitor.Start(dir); err != nil {
		log.Fatalf("Failed to start monitoring: %v", err)
	}

	// Print events as they occur
	go func() {
		for event := range monitor.events {
			fmt.Printf("[%s] %s: %s\n",
				event.Timestamp.Format("15:04:05"),
				event.Operation,
				event.Path)
		}
	}()

	// Wait for Ctrl+C
	fmt.Println("Press Ctrl+C to stop monitoring")
	ch := make(chan os.Signal, 1)
	<-ch

	// Clean up
	monitor.Stop()
}
