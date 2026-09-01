package projects

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

type Catalog struct {
	Path     string
	LockPath string
}

func NewCatalog() *Catalog {
	return &Catalog{Path: DefaultCatalogPath, LockPath: DefaultLockPath}
}

func (catalog *Catalog) List() ([]CatalogEntry, error) {
	return catalog.listUnlocked()
}

func (catalog *Catalog) listUnlocked() ([]CatalogEntry, error) {
	contents, err := os.ReadFile(catalog.Path)
	if errors.Is(err, os.ErrNotExist) {
		return []CatalogEntry{}, nil
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

func (catalog *Catalog) Get(id string) (CatalogEntry, error) {
	entries, err := catalog.List()
	if err != nil {
		return CatalogEntry{}, err
	}
	for _, entry := range entries {
		if entry.ID == id {
			return entry, nil
		}
	}
	return CatalogEntry{}, fmt.Errorf("project %q does not exist", id)
}

func (catalog *Catalog) Add(entry CatalogEntry) error {
	if err := entry.Validate(); err != nil {
		return err
	}
	return catalog.mutate(func(entries []CatalogEntry) ([]CatalogEntry, error) {
		for _, current := range entries {
			if current.ID == entry.ID {
				return nil, fmt.Errorf("project %q already exists", entry.ID)
			}
		}
		return append(entries, entry), nil
	})
}

func (catalog *Catalog) Edit(entry CatalogEntry) error {
	if err := entry.Validate(); err != nil {
		return err
	}
	return catalog.mutate(func(entries []CatalogEntry) ([]CatalogEntry, error) {
		for index := range entries {
			if entries[index].ID == entry.ID {
				entries[index] = entry
				return entries, nil
			}
		}
		return nil, fmt.Errorf("project %q does not exist", entry.ID)
	})
}

func (catalog *Catalog) Remove(id string) error {
	if !projectIDPattern.MatchString(id) {
		return errors.New("project id must match [a-z][a-z0-9-]{0,23}")
	}
	return catalog.Exclusive(func() error { return catalog.removeUnlocked(id) })
}

func (catalog *Catalog) mutate(change func([]CatalogEntry) ([]CatalogEntry, error)) error {
	return catalog.Exclusive(func() error { return catalog.mutateUnlocked(change) })
}

func (catalog *Catalog) mutateUnlocked(change func([]CatalogEntry) ([]CatalogEntry, error)) error {
	entries, err := catalog.listUnlocked()
	if err != nil {
		return err
	}
	entries, err = change(entries)
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	return catalog.write(entries)
}

// Exclusive serializes one complete root-owned catalog/workspace lifecycle
// mutation. The callback must use unlocked catalog helpers to avoid re-locking.
func (catalog *Catalog) Exclusive(operation func() error) error {
	lock, err := catalog.lock()
	if err != nil {
		return err
	}
	defer lock.Close()
	return operation()
}

func (catalog *Catalog) removeUnlocked(id string) error {
	return catalog.mutateUnlocked(func(entries []CatalogEntry) ([]CatalogEntry, error) {
		for index := range entries {
			if entries[index].ID == id {
				return append(entries[:index:index], entries[index+1:]...), nil
			}
		}
		return nil, fmt.Errorf("project %q does not exist", id)
	})
}

func (catalog *Catalog) lock() (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(catalog.LockPath), 0o755); err != nil {
		return nil, fmt.Errorf("create project lock directory: %w", err)
	}
	file, err := os.OpenFile(catalog.LockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open project lock: %w", err)
	}
	if err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		file.Close()
		return nil, fmt.Errorf("lock project catalog: %w", err)
	}
	return file, nil
}

func (catalog *Catalog) write(entries []CatalogEntry) error {
	directory := filepath.Dir(catalog.Path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create project catalog directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".projects-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary project catalog: %w", err)
	}
	temporaryName := temporary.Name()
	if err = populateCatalogTemporary(temporary, entries); err != nil {
		return discardOpenCatalogTemporary(temporary, temporaryName, err)
	}
	if err = temporary.Close(); err != nil {
		return discardCatalogTemporary(temporaryName, fmt.Errorf("close project catalog: %w", err))
	}
	if err = os.Rename(temporaryName, catalog.Path); err != nil {
		return discardCatalogTemporary(temporaryName, fmt.Errorf("publish project catalog: %w", err))
	}
	return syncCatalogDirectory(directory)
}

func populateCatalogTemporary(temporary *os.File, entries []CatalogEntry) error {
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

func discardOpenCatalogTemporary(temporary *os.File, name string, cause error) error {
	closeErr := temporary.Close()
	if closeErr != nil {
		closeErr = fmt.Errorf("close project catalog: %w", closeErr)
	}
	return errors.Join(cause, closeErr, removeCatalogTemporary(name))
}

func discardCatalogTemporary(name string, cause error) error {
	return errors.Join(cause, removeCatalogTemporary(name))
}

func removeCatalogTemporary(name string) error {
	err := os.Remove(name)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return fmt.Errorf("remove temporary project catalog: %w", err)
}

func syncCatalogDirectory(directory string) error {
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

func decodeCatalog(reader io.Reader) ([]CatalogEntry, error) {
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
	entries, err := decodeCatalogEntries(decoder)
	if err != nil {
		return nil, err
	}
	if err = finishJSONArray(decoder); err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return strings.Compare(entries[i].ID, entries[j].ID) < 0 })
	return entries, nil
}

func decodeCatalogEntries(decoder *json.Decoder) ([]CatalogEntry, error) {
	entries := []CatalogEntry{}
	seenIDs := map[string]bool{}
	for decoder.More() {
		entry, err := decodeCatalogEntry(decoder)
		if err != nil {
			return nil, err
		}
		if err = entry.Validate(); err != nil {
			return nil, fmt.Errorf("project %q: %w", entry.ID, err)
		}
		if seenIDs[entry.ID] {
			return nil, fmt.Errorf("duplicate project id %q", entry.ID)
		}
		seenIDs[entry.ID] = true
		entries = append(entries, entry)
	}
	return entries, nil
}

func finishJSONArray(decoder *json.Decoder) error {
	if err := requireJSONDelimiter(decoder, ']', "catalog array is not closed"); err != nil {
		return err
	}
	token, err := decoder.Token()
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("unexpected JSON value %v after catalog", token)
}

func decodeCatalogEntry(decoder *json.Decoder) (CatalogEntry, error) {
	if err := requireJSONDelimiter(decoder, '{', "catalog entry must be a JSON object"); err != nil {
		return CatalogEntry{}, err
	}
	values, err := decodeCatalogFields(decoder)
	if err != nil {
		return CatalogEntry{}, err
	}
	if err = requireJSONDelimiter(decoder, '}', "catalog entry object is not closed"); err != nil {
		return CatalogEntry{}, err
	}
	return catalogEntryFromValues(values)
}

func decodeCatalogFields(decoder *json.Decoder) (map[string]string, error) {
	values := map[string]string{}
	for decoder.More() {
		field, err := decodeCatalogFieldName(decoder, values)
		if err != nil {
			return nil, err
		}
		var value string
		if err := decoder.Decode(&value); err != nil {
			return nil, fmt.Errorf("catalog field %q must be a string", field)
		}
		values[field] = value
	}
	return values, nil
}

func decodeCatalogFieldName(decoder *json.Decoder, values map[string]string) (string, error) {
	token, err := decoder.Token()
	if err != nil {
		return "", err
	}
	field, ok := token.(string)
	if !ok {
		return "", errors.New("catalog field name must be a string")
	}
	if field != "id" && field != "display_name" && field != "canonical_url" {
		return "", fmt.Errorf("unknown catalog field %q", field)
	}
	if _, duplicate := values[field]; duplicate {
		return "", fmt.Errorf("duplicate catalog field %q", field)
	}
	return field, nil
}

func catalogEntryFromValues(values map[string]string) (CatalogEntry, error) {
	for _, field := range []string{"id", "display_name", "canonical_url"} {
		if _, present := values[field]; !present {
			return CatalogEntry{}, fmt.Errorf("catalog entry is missing %q", field)
		}
	}
	return CatalogEntry{ID: values["id"], DisplayName: values["display_name"], CanonicalURL: values["canonical_url"]}, nil
}

func requireJSONDelimiter(decoder *json.Decoder, expected json.Delim, message string) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok || delimiter != expected {
		return errors.New(message)
	}
	return nil
}
