package catalog

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"unicode/utf8"
)

const (
	SystemPath     = "/var/lib/soda/catalog/projects.json"
	SystemLockPath = "/run/lock/soda/projects.lock"
)

// Store provides atomic, process-safe access to one project catalog.
type Store struct {
	path     string
	lockPath string
}

// NewSystemStore constructs the production catalog at Soda's fixed paths.
func NewSystemStore() *Store {
	return &Store{path: SystemPath, lockPath: SystemLockPath}
}

// NewStore constructs a catalog at explicit paths, principally for tests and
// installed-environment fixtures.
func NewStore(path, lockPath string) (*Store, error) {
	if path == "" {
		return nil, errors.New("project catalog path is required")
	}
	if lockPath == "" {
		return nil, errors.New("project catalog lock path is required")
	}
	return &Store{path: path, lockPath: lockPath}, nil
}

func (store *Store) List() ([]Entry, error) {
	if err := store.requireConfigured(); err != nil {
		return nil, err
	}
	return store.listUnlocked()
}

func (store *Store) Get(id string) (Entry, error) {
	if err := store.requireConfigured(); err != nil {
		return Entry{}, err
	}
	return store.getUnlocked(id)
}

func (store *Store) Add(entry Entry) error {
	locked, err := store.Lock()
	if err != nil {
		return err
	}
	operationErr := locked.Add(entry)
	return errors.Join(operationErr, locked.Close())
}

func (store *Store) Edit(edit Edit) error {
	locked, err := store.Lock()
	if err != nil {
		return err
	}
	_, operationErr := locked.Edit(edit)
	return errors.Join(operationErr, locked.Close())
}

func (locked *LockedStore) Add(entry Entry) error {
	if err := locked.requireOpen(); err != nil {
		return err
	}
	if err := entry.Validate(); err != nil {
		return err
	}
	return locked.mutate(func(entries []Entry) ([]Entry, error) {
		for _, current := range entries {
			if current.ID == entry.ID {
				return nil, fmt.Errorf("project %q already exists", entry.ID)
			}
		}
		return append(entries, entry), nil
	})
}

func (locked *LockedStore) Edit(edit Edit) (Entry, error) {
	if err := locked.requireOpen(); err != nil {
		return Entry{}, err
	}
	if err := edit.Validate(); err != nil {
		return Entry{}, err
	}
	var updated Entry
	err := locked.mutate(func(entries []Entry) ([]Entry, error) {
		for index := range entries {
			if entries[index].ID == edit.ID {
				updated = edit.Apply(entries[index])
				entries[index] = updated
				return entries, nil
			}
		}
		return nil, fmt.Errorf("project %q does not exist", edit.ID)
	})
	return updated, err
}

// Lock acquires the catalog lock for a caller that must keep workspace removal
// and catalog removal in one ordered operation. The caller must close the
// returned LockedStore.
func (store *Store) Lock() (*LockedStore, error) {
	if err := store.requireConfigured(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(store.lockPath), 0o755); err != nil {
		return nil, fmt.Errorf("create project lock directory: %w", err)
	}
	file, err := os.OpenFile(store.lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open project lock: %w", err)
	}
	if err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return nil, errors.Join(fmt.Errorf("lock project catalog: %w", err), file.Close())
	}
	return &LockedStore{store: store, lock: file}, nil
}

func (store *Store) requireConfigured() error {
	if store == nil || store.path == "" || store.lockPath == "" {
		return errors.New("project catalog store was not constructed")
	}
	return nil
}

// LockedStore exposes only catalog mutations that must remain under one
// caller-owned catalog lock.
type LockedStore struct {
	store  *Store
	lock   *os.File
	closed bool
}

func (locked *LockedStore) Get(id string) (Entry, error) {
	if err := locked.requireOpen(); err != nil {
		return Entry{}, err
	}
	return locked.store.getUnlocked(id)
}

func (locked *LockedStore) Remove(id string) error {
	if err := locked.requireOpen(); err != nil {
		return err
	}
	if err := ValidateID(id); err != nil {
		return err
	}
	return locked.mutate(func(entries []Entry) ([]Entry, error) {
		for index := range entries {
			if entries[index].ID == id {
				return append(entries[:index:index], entries[index+1:]...), nil
			}
		}
		return nil, fmt.Errorf("project %q does not exist", id)
	})
}

func (locked *LockedStore) Close() error {
	if locked == nil || locked.closed {
		return nil
	}
	locked.closed = true
	unlockErr := syscall.Flock(int(locked.lock.Fd()), syscall.LOCK_UN)
	if unlockErr != nil {
		unlockErr = fmt.Errorf("unlock project catalog: %w", unlockErr)
	}
	closeErr := locked.lock.Close()
	if closeErr != nil {
		closeErr = fmt.Errorf("close project catalog lock: %w", closeErr)
	}
	return errors.Join(unlockErr, closeErr)
}

func (locked *LockedStore) requireOpen() error {
	if locked == nil || locked.closed {
		return errors.New("project catalog lock is closed")
	}
	return nil
}

func (locked *LockedStore) mutate(change func([]Entry) ([]Entry, error)) error {
	entries, err := locked.store.listUnlocked()
	if err != nil {
		return err
	}
	entries, err = change(entries)
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	return locked.store.write(entries)
}

func (store *Store) getUnlocked(id string) (Entry, error) {
	entries, err := store.listUnlocked()
	if err != nil {
		return Entry{}, err
	}
	for _, entry := range entries {
		if entry.ID == id {
			return entry, nil
		}
	}
	return Entry{}, fmt.Errorf("project %q does not exist", id)
}

func (store *Store) listUnlocked() ([]Entry, error) {
	contents, err := os.ReadFile(store.path)
	if errors.Is(err, os.ErrNotExist) {
		return []Entry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read project catalog: %w", err)
	}
	entries, err := decodeCatalog(bytes.NewReader(contents))
	if err != nil {
		return nil, fmt.Errorf("read project catalog: %w", err)
	}
	return entries, nil
}

func (store *Store) write(entries []Entry) error {
	directory := filepath.Dir(store.path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create project catalog directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".projects-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary project catalog: %w", err)
	}
	temporaryName := temporary.Name()
	if err = populateTemporary(temporary, entries); err != nil {
		return discardOpenTemporary(temporary, temporaryName, err)
	}
	if err = temporary.Close(); err != nil {
		return discardTemporary(temporaryName, fmt.Errorf("close project catalog: %w", err))
	}
	if err = os.Rename(temporaryName, store.path); err != nil {
		return discardTemporary(temporaryName, fmt.Errorf("publish project catalog: %w", err))
	}
	return syncDirectory(directory)
}

func populateTemporary(temporary *os.File, entries []Entry) error {
	if err := temporary.Chmod(0o644); err != nil {
		return fmt.Errorf("protect project catalog: %w", err)
	}
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(entries); err != nil {
		return fmt.Errorf("encode project catalog: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync project catalog: %w", err)
	}
	return nil
}

func discardOpenTemporary(temporary *os.File, name string, cause error) error {
	closeErr := temporary.Close()
	if closeErr != nil {
		closeErr = fmt.Errorf("close project catalog: %w", closeErr)
	}
	return errors.Join(cause, closeErr, removeTemporary(name))
}

func discardTemporary(name string, cause error) error {
	return errors.Join(cause, removeTemporary(name))
}

func removeTemporary(name string) error {
	err := os.Remove(name)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return fmt.Errorf("remove temporary project catalog: %w", err)
}

func syncDirectory(directory string) error {
	directoryFile, err := os.Open(directory)
	if err != nil {
		return fmt.Errorf("open project catalog directory: %w", err)
	}
	defer directoryFile.Close()
	if err = directoryFile.Sync(); err != nil {
		return fmt.Errorf("sync project catalog directory: %w", err)
	}
	return nil
}

func decodeCatalog(reader io.Reader) ([]Entry, error) {
	contents, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(contents) {
		return nil, errors.New("catalog must contain valid UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	if err := requireJSONDelimiter(decoder, '[', "catalog must be a JSON array"); err != nil {
		return nil, err
	}
	entries := []Entry{}
	seenIDs := map[string]bool{}
	for decoder.More() {
		entry, decodeErr := decodeEntry(decoder)
		if decodeErr != nil {
			return nil, decodeErr
		}
		if seenIDs[entry.ID] {
			return nil, fmt.Errorf("duplicate project id %q", entry.ID)
		}
		seenIDs[entry.ID] = true
		entries = append(entries, entry)
	}
	if err = requireJSONDelimiter(decoder, ']', "catalog array is not closed"); err != nil {
		return nil, err
	}
	if err = requireEOF(decoder, "catalog"); err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return strings.Compare(entries[i].ID, entries[j].ID) < 0 })
	return entries, nil
}
