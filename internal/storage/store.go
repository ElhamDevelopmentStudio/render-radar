package storage

import (
	"debugger-api/internal/debugger"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type DebugSession struct {
    Version     int                           `json:"version"`
    Timestamp   time.Time                     `json:"timestamp"`
    Results     map[string]debugger.PageResults `json:"results"`
    Errors      map[string]string            `json:"errors"`
}

type Store struct {
    mu       sync.RWMutex
    sessions map[string][]DebugSession // URL -> []Sessions
    dataDir  string
}

func NewStore(dataDir string) (*Store, error) {
    if err := os.MkdirAll(dataDir, 0755); err != nil {
        return nil, err
    }

    store := &Store{
        sessions: make(map[string][]DebugSession),
        dataDir:  dataDir,
    }

    // Load existing sessions
    if err := store.loadSessions(); err != nil {
        return nil, err
    }

    return store, nil
}

func (s *Store) SaveSession(url string, results debugger.PageResults, errors map[string]string) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    fmt.Printf("💾 Saving session for %s\n", url)
    sessions := s.sessions[url]
    newVersion := len(sessions) + 1

    session := DebugSession{
        Version:   newVersion,
        Timestamp: time.Now(),
        Results:   map[string]debugger.PageResults{url: results},
        Errors:    errors,
    }

    s.sessions[url] = append(s.sessions[url], session)
    return s.persist()
}

func (s *Store) GetSessions(url string) []DebugSession {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.sessions[url]
}

func (s *Store) ClearSessions(url string) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    delete(s.sessions, url)
    return s.persist()
}

func (s *Store) ClearAllSessions() error {
    s.mu.Lock()
    defer s.mu.Unlock()

    fmt.Println("🗑️ Clearing all sessions...")
    s.sessions = make(map[string][]DebugSession)
    
    // Remove the data file
    dataFile := filepath.Join(s.dataDir, "sessions.json")
    if err := os.Remove(dataFile); err != nil && !os.IsNotExist(err) {
        fmt.Printf("❌ Failed to remove data file: %v\n", err)
        return err
    }
    
    fmt.Println("✅ All sessions cleared")
    return nil
}

func (s *Store) Cleanup() error {
    s.mu.Lock()
    defer s.mu.Unlock()

    // Clear memory
    s.sessions = make(map[string][]DebugSession)
    
    // Remove the data file
    if err := os.Remove(filepath.Join(s.dataDir, "sessions.json")); err != nil && !os.IsNotExist(err) {
        return err
    }
    
    return nil
}

func (s *Store) persist() error {
    data, err := json.MarshalIndent(s.sessions, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(filepath.Join(s.dataDir, "sessions.json"), data, 0644)
}

func (s *Store) loadSessions() error {
    path := filepath.Join(s.dataDir, "sessions.json")
    if _, err := os.Stat(path); os.IsNotExist(err) {
        return nil
    }

    data, err := os.ReadFile(path)
    if err != nil {
        return err
    }

    return json.Unmarshal(data, &s.sessions)
} 