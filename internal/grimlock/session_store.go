package grimlock

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// sessionStore persists Grimlock session metadata to a directory of JSON
// files so that session summaries and event history survive process restarts.
// The active runner, agent, and attachment state are not persisted; callers
// must reconnect model gateways and tools before resuming a loaded session.
type sessionStore struct {
	mu       sync.Mutex
	dir      string
	sessions map[string]*runningSession
}

type sessionRecord struct {
	Summary SessionView     `json:"summary"`
	Events  []EventEnvelope `json:"events,omitempty"`
	Status  string          `json:"status"`
	Closed  bool            `json:"closed"`
}

func newSessionStore(dir string) (*sessionStore, error) {
	if dir == "" {
		return nil, errors.New("session store directory is required")
	}
	store := &sessionStore{
		dir:      dir,
		sessions: make(map[string]*runningSession),
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create session store directory: %w", err)
	}
	if err := store.loadAll(); err != nil {
		return nil, err
	}
	return store, nil
}

func (st *sessionStore) loadAll() error {
	entries, err := os.ReadDir(st.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read session store: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(st.dir, entry.Name()))
		if err != nil {
			continue // skip unreadable files
		}
		var rec sessionRecord
		if err := json.Unmarshal(raw, &rec); err != nil {
			continue // skip invalid files
		}
		st.sessions[rec.Summary.SessionID] = &runningSession{
			summary: rec.Summary,
			status:  rec.Status,
			closed:  rec.Closed,
			events:  rec.Events,
			pending: make(map[string]PendingApproval),
		}
		if rec.Summary.PendingApprovals != nil {
			for _, approval := range rec.Summary.PendingApprovals {
				st.sessions[rec.Summary.SessionID].pending[approval.ID] = approval
			}
		}
		st.sessions[rec.Summary.SessionID].nextCursor = uint64(len(rec.Events))
	}
	return nil
}

func (st *sessionStore) recordSession(rec *runningSession) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.writeLocked(rec)
}

func (st *sessionStore) writeLocked(rec *runningSession) error {
	st.sessions[rec.summary.SessionID] = rec
	return st.writeFileLocked(rec)
}

func (st *sessionStore) writeFileLocked(rec *runningSession) error {
	rec.mu.Lock()
	record := sessionRecord{
		Summary: rec.summary,
		Events:  append([]EventEnvelope(nil), rec.events...),
		Status:  rec.status,
		Closed:  rec.closed,
	}
	rec.mu.Unlock()

	raw, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal session record: %w", err)
	}
	path := filepath.Join(st.dir, sessionFileName(rec.summary.SessionID))
	if err := os.WriteFile(path, raw, 0600); err != nil {
		return fmt.Errorf("write session file: %w", err)
	}
	return nil
}

func (st *sessionStore) deleteSession(id string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	delete(st.sessions, id)
	path := filepath.Join(st.dir, sessionFileName(id))
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove session file: %w", err)
	}
	return nil
}

func sessionFileName(id string) string {
	clean := make([]byte, 0, len(id))
	for _, b := range []byte(id) {
		if (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '-' || b == '_' {
			clean = append(clean, b)
		}
	}
	if len(clean) == 0 {
		return "unknown.session.json"
	}
	return string(clean) + ".session.json"
}

// loadSession returns a runningSession from the store if it was persisted.
func (st *sessionStore) loadSession(id string) (*runningSession, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	rec, ok := st.sessions[id]
	if !ok {
		return nil, false
	}
	return rec, true
}

// lookupFromStore implements the store interface for compatibility with
// the existing lookupSession pattern.
func (s *Service) lookupFromStore(id string) (*runningSession, bool) {
	if s.store == nil {
		return nil, false
	}
	return s.store.loadSession(id)
}

var _ = (*Service).lookupFromStore

func (s *Service) saveSession(rec *runningSession) error {
	if s.store == nil {
		return nil
	}
	return s.store.recordSession(rec)
}

func (s *Service) deleteStoredSession(id string) error {
	if s.store == nil {
		return nil
	}
	return s.store.deleteSession(id)
}
