package db

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// sqliteHeaderMagic is the fixed 16-byte header every well-formed SQLite
// database file begins with.
const sqliteHeaderMagic = "SQLite format 3\x00"

// RegistryEntry describes a single sandbox-managed SQLite database.
type RegistryEntry struct {
	ID             string
	Path           string
	DisplayName    string
	DB             *sql.DB
	OpenedAt       time.Time
	CreatedAt      time.Time
	LastModifiedAt time.Time
	SizeBytes      int64
}

// Registry tracks a set of independently opened *sql.DB connections backed
// by files under a managed directory, keyed by an opaque generated ID. It is
// additive to the single-*sql.DB flow used by `squad <db>` — it does not
// replace it.
type Registry struct {
	mu          sync.RWMutex
	dir         string
	maxUploadMB int64
	entries     map[string]*RegistryEntry
}

// NewRegistry creates a Registry rooted at dir, enforcing maxUploadBytes on
// uploads.
func NewRegistry(dir string, maxUploadBytes int64) *Registry {
	return &Registry{
		dir:         dir,
		maxUploadMB: maxUploadBytes,
		entries:     make(map[string]*RegistryEntry),
	}
}

// Dir returns the resolved directory backing this registry.
func (r *Registry) Dir() string {
	return r.dir
}

// MaxUploadBytes returns the configured max upload size in bytes.
func (r *Registry) MaxUploadBytes() int64 {
	return r.maxUploadMB
}

func newID() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// looksLikeSQLite reads the first 16 bytes from r and checks them against the
// SQLite file header magic. It returns a reader that reproduces the full
// original stream (including the bytes it consumed for the check) so the
// caller can still write everything to disk, since the underlying reader
// (e.g. a multipart upload stream) isn't guaranteed to be seekable.
func looksLikeSQLite(r io.Reader) (rest io.Reader, ok bool, err error) {
	header := make([]byte, len(sqliteHeaderMagic))
	n, readErr := io.ReadFull(r, header)
	combined := io.MultiReader(bytes.NewReader(header[:n]), r)
	if readErr != nil && readErr != io.ErrUnexpectedEOF && readErr != io.EOF {
		return combined, false, readErr
	}
	if n < len(header) || string(header) != sqliteHeaderMagic {
		return combined, false, nil
	}
	return combined, true, nil
}

// allocID generates a fresh, currently-unused ID. Caller must hold r.mu (at
// least for the final registration), but generation itself doesn't need the
// lock since collisions are checked again at registration time.
func (r *Registry) allocID() (string, error) {
	const maxAttempts = 5
	for i := 0; i < maxAttempts; i++ {
		id, err := newID()
		if err != nil {
			return "", err
		}
		r.mu.RLock()
		_, taken := r.entries[id]
		r.mu.RUnlock()
		if !taken {
			return id, nil
		}
	}
	return "", fmt.Errorf("failed to allocate a unique sandbox id after %d attempts", maxAttempts)
}

// Add streams src to a new file under the registry directory, validates it
// looks like a well-formed SQLite database, opens it read-write, and
// registers it under a freshly generated ID. On any failure the partially
// written file (if any) is removed.
func (r *Registry) Add(displayName string, src io.Reader) (*RegistryEntry, error) {
	id, err := r.allocID()
	if err != nil {
		return nil, err
	}

	checkedSrc, ok, err := looksLikeSQLite(src)
	if err != nil {
		return nil, fmt.Errorf("failed to read uploaded file: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("uploaded file is not a valid SQLite database")
	}

	path := filepath.Join(r.dir, id+".db")
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create sandbox db file: %w", err)
	}
	if _, err := io.Copy(f, checkedSrc); err != nil {
		f.Close()
		os.Remove(path)
		return nil, fmt.Errorf("failed to write sandbox db file: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return nil, fmt.Errorf("failed to finalize sandbox db file: %w", err)
	}

	return r.registerOpened(id, path, displayName)
}

// Create makes a new, empty SQLite database file under the registry
// directory and registers it under a freshly generated ID.
func (r *Registry) Create(displayName string) (*RegistryEntry, error) {
	id, err := r.allocID()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(r.dir, id+".db")
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create sandbox db file: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return nil, fmt.Errorf("failed to finalize sandbox db file: %w", err)
	}
	return r.registerOpened(id, path, displayName)
}

// registerOpened opens path read-write and inserts a new entry into the
// registry under id, cleaning up the file on failure.
func (r *Registry) registerOpened(id, path, displayName string) (*RegistryEntry, error) {
	database, err := OpenDB(path, false)
	if err != nil {
		os.Remove(path)
		return nil, fmt.Errorf("failed to open sandbox db: %w", err)
	}

	now := time.Now()
	size := int64(0)
	if info, statErr := os.Stat(path); statErr == nil {
		size = info.Size()
	}

	entry := &RegistryEntry{
		ID:             id,
		Path:           path,
		DisplayName:    displayName,
		DB:             database,
		OpenedAt:       now,
		CreatedAt:      now,
		LastModifiedAt: now,
		SizeBytes:      size,
	}

	r.mu.Lock()
	if _, taken := r.entries[id]; taken {
		r.mu.Unlock()
		database.Close()
		os.Remove(path)
		return nil, fmt.Errorf("sandbox id collision for %q", id)
	}
	r.entries[id] = entry
	r.mu.Unlock()

	return entry, nil
}

// Get returns the entry for id, if present.
func (r *Registry) Get(id string) (*RegistryEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.entries[id]
	return entry, ok
}

// List returns a snapshot of all registered entries, sorted by CreatedAt
// ascending. SizeBytes is refreshed from disk for each entry.
func (r *Registry) List() []RegistryEntry {
	r.mu.RLock()
	out := make([]RegistryEntry, 0, len(r.entries))
	for _, e := range r.entries {
		snapshot := *e
		if info, err := os.Stat(e.Path); err == nil {
			snapshot.SizeBytes = info.Size()
			snapshot.LastModifiedAt = info.ModTime()
		}
		out = append(out, snapshot)
	}
	r.mu.RUnlock()

	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

// Rename updates the display name of an entry. It never touches the on-disk
// filename or path.
func (r *Registry) Rename(id, newDisplayName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.entries[id]
	if !ok {
		return fmt.Errorf("sandbox db %q not found", id)
	}
	entry.DisplayName = newDisplayName
	return nil
}

// Touch updates LastModifiedAt for id. It is a no-op if the id is unknown.
func (r *Registry) Touch(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry, ok := r.entries[id]; ok {
		entry.LastModifiedAt = time.Now()
	}
}

// Remove closes the entry's connection, deletes its backing file, and
// removes it from the registry.
func (r *Registry) Remove(id string) error {
	r.mu.Lock()
	entry, ok := r.entries[id]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("sandbox db %q not found", id)
	}
	delete(r.entries, id)
	r.mu.Unlock()

	entry.DB.Close()
	if err := os.Remove(entry.Path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove sandbox db file: %w", err)
	}
	return nil
}

// Rescan walks the registry directory for *.db files not already registered,
// validates and opens each, and registers the valid ones. Files that fail to
// open or validate are skipped and reported in the returned error slice;
// this never fails the overall scan.
func (r *Registry) Rescan() []error {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return []error{fmt.Errorf("failed to read sandbox dir: %w", err)}
	}

	var errs []error
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".db") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".db")

		r.mu.RLock()
		_, already := r.entries[id]
		r.mu.RUnlock()
		if already {
			continue
		}

		path := filepath.Join(r.dir, e.Name())

		// A brand-new empty SQLite database is a legitimate zero-byte file
		// (SQLite doesn't allocate any pages until the first write), which
		// is exactly what Registry.Create produces — skip the header check
		// for empty files rather than flagging our own output as invalid.
		if info, statErr := e.Info(); statErr == nil && info.Size() > 0 {
			f, err := os.Open(path)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", e.Name(), err))
				continue
			}
			_, ok, err := looksLikeSQLite(f)
			f.Close()
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", e.Name(), err))
				continue
			}
			if !ok {
				errs = append(errs, fmt.Errorf("%s: not a valid SQLite database", e.Name()))
				continue
			}
		}

		database, err := OpenDB(path, false)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", e.Name(), err))
			continue
		}

		info, statErr := os.Stat(path)
		now := time.Now()
		createdAt, modAt := now, now
		size := int64(0)
		if statErr == nil {
			createdAt = info.ModTime()
			modAt = info.ModTime()
			size = info.Size()
		}

		r.mu.Lock()
		r.entries[id] = &RegistryEntry{
			ID:             id,
			Path:           path,
			DisplayName:    id,
			DB:             database,
			OpenedAt:       now,
			CreatedAt:      createdAt,
			LastModifiedAt: modAt,
			SizeBytes:      size,
		}
		r.mu.Unlock()
	}
	return errs
}

// CloseAll closes every registered connection. It does not delete any files.
func (r *Registry) CloseAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.entries {
		e.DB.Close()
	}
}
