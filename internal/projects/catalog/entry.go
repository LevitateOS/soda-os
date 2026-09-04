// Package catalog owns Soda's minimal appliance-wide project catalog.
package catalog

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var projectIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,23}$`)

// Entry is the complete durable Soda project representation. Additional
// fields remain raw JSON so the catalog does not assume a closed metadata
// schema.
type Entry struct {
	ID           string                     `json:"id"`
	DisplayName  string                     `json:"display_name"`
	CanonicalURL string                     `json:"canonical_url"`
	Additional   map[string]json.RawMessage `json:"-"`
}

func (entry Entry) Validate() error {
	if err := validateIdentity(entry.ID, entry.DisplayName); err != nil {
		return err
	}
	if err := ValidateCanonicalURL(entry.CanonicalURL); err != nil {
		return fmt.Errorf("canonical URL: %w", err)
	}
	return validateAdditionalFields(entry.Additional)
}

func ValidateID(id string) error {
	if !projectIDPattern.MatchString(id) {
		return errors.New("project id must match [a-z][a-z0-9-]{0,23}")
	}
	return nil
}

func validateIdentity(id, displayName string) error {
	if err := ValidateID(id); err != nil {
		return err
	}
	if displayName == "" || !utf8.ValidString(displayName) {
		return errors.New("display name must be non-empty UTF-8")
	}
	if strings.IndexFunc(displayName, unicode.IsControl) >= 0 {
		return errors.New("display name must not contain control characters")
	}
	return nil
}

func validateAdditionalFields(additional map[string]json.RawMessage) error {
	for field, value := range additional {
		if field == "" || !utf8.ValidString(field) {
			return errors.New("additional catalog field names must be non-empty UTF-8")
		}
		if isRequiredField(field) {
			return fmt.Errorf("additional catalog field %q conflicts with a required field", field)
		}
		if !json.Valid(value) {
			return fmt.Errorf("additional catalog field %q must contain valid JSON", field)
		}
	}
	return nil
}

func (entry Entry) jsonObject() map[string]json.RawMessage {
	object := cloneAdditional(entry.Additional, 3)
	object["id"], _ = json.Marshal(entry.ID)
	object["display_name"], _ = json.Marshal(entry.DisplayName)
	object["canonical_url"], _ = json.Marshal(entry.CanonicalURL)
	return object
}

func (entry Entry) MarshalJSON() ([]byte, error) {
	if err := entry.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(entry.jsonObject())
}

func (entry *Entry) UnmarshalJSON(contents []byte) error {
	decoded, err := decodeSingleEntry(contents)
	if err != nil {
		return err
	}
	*entry = decoded
	return nil
}

// Edit changes an entry's display name and arbitrary metadata. It deliberately
// has no canonical URL field: replacing that immutable identity requires
// removing and re-adding the project.
type Edit struct {
	ID          string                     `json:"id"`
	DisplayName string                     `json:"display_name"`
	Additional  map[string]json.RawMessage `json:"-"`
}

func (edit Edit) Validate() error {
	if err := validateIdentity(edit.ID, edit.DisplayName); err != nil {
		return err
	}
	return validateAdditionalFields(edit.Additional)
}

func (edit Edit) Apply(current Entry) Entry {
	additional := edit.Additional
	if additional == nil {
		additional = current.Additional
	}
	return Entry{
		ID:           current.ID,
		DisplayName:  edit.DisplayName,
		CanonicalURL: current.CanonicalURL,
		Additional:   cloneAdditionalOrNil(additional),
	}
}

func (edit Edit) MarshalJSON() ([]byte, error) {
	if err := edit.Validate(); err != nil {
		return nil, err
	}
	object := cloneAdditional(edit.Additional, 2)
	object["id"], _ = json.Marshal(edit.ID)
	object["display_name"], _ = json.Marshal(edit.DisplayName)
	return json.Marshal(object)
}

func (edit *Edit) UnmarshalJSON(contents []byte) error {
	values, err := decodeSingleObject(contents, "edit request")
	if err != nil {
		return err
	}
	decoded, err := editFromValues(values)
	if err != nil {
		return err
	}
	*edit = decoded
	return nil
}

func editFromValues(values map[string]json.RawMessage) (Edit, error) {
	if _, present := values["canonical_url"]; present {
		return Edit{}, errors.New(`edit request must not include "canonical_url"; remove and re-add the project to replace its canonical URL`)
	}
	for _, field := range []string{"id", "display_name"} {
		if _, present := values[field]; !present {
			return Edit{}, fmt.Errorf("edit request is missing %q", field)
		}
	}
	edit := Edit{Additional: map[string]json.RawMessage{}}
	for field, value := range values {
		var target *string
		switch field {
		case "id":
			target = &edit.ID
		case "display_name":
			target = &edit.DisplayName
		default:
			edit.Additional[field] = append(json.RawMessage(nil), value...)
			continue
		}
		if err := json.Unmarshal(value, target); err != nil {
			return Edit{}, fmt.Errorf("edit field %q must be a string", field)
		}
	}
	if err := edit.Validate(); err != nil {
		return Edit{}, err
	}
	return edit, nil
}

func cloneAdditional(additional map[string]json.RawMessage, requiredFields int) map[string]json.RawMessage {
	object := make(map[string]json.RawMessage, len(additional)+requiredFields)
	for field, value := range additional {
		object[field] = append(json.RawMessage(nil), value...)
	}
	return object
}

func cloneAdditionalOrNil(additional map[string]json.RawMessage) map[string]json.RawMessage {
	if additional == nil {
		return nil
	}
	return cloneAdditional(additional, 0)
}

func ValidateCanonicalURL(remote string) error {
	if err := validateRemoteText(remote); err != nil {
		return err
	}
	if validSCPLikeRemote(remote) {
		return nil
	}
	parsed, err := url.Parse(remote)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	return validateStructuredRemote(parsed)
}

func validateRemoteText(remote string) error {
	switch {
	case remote == "":
		return errors.New("is required")
	case strings.HasPrefix(strings.ToLower(remote), "file:"):
		return errors.New("must not use a file URL or local file syntax")
	case driveLetterPath(remote):
		return errors.New("must not use a local drive path")
	case strings.ContainsAny(remote, "?#"):
		return errors.New("must not contain a query or fragment")
	case strings.IndexFunc(remote, unicode.IsSpace) >= 0:
		return errors.New("must not contain whitespace")
	case strings.IndexFunc(remote, unicode.IsControl) >= 0:
		return errors.New("must not contain control characters")
	default:
		return nil
	}
}

func driveLetterPath(remote string) bool {
	if len(remote) < 3 || remote[1] != ':' {
		return false
	}
	letter := remote[0]
	return ((letter >= 'a' && letter <= 'z') || (letter >= 'A' && letter <= 'Z')) && (remote[2] == '/' || remote[2] == '\\')
}

func validateStructuredRemote(parsed *url.URL) error {
	if parsed.Host == "" || parsed.Path == "" || parsed.Path == "/" {
		return errors.New("must include a host and repository path")
	}
	if strings.ToLower(parsed.Scheme) != "ssh" {
		return errors.New("must use SSH or SCP syntax")
	}
	if parsed.User == nil {
		return nil
	}
	if parsed.User.Username() == "" {
		return errors.New("SSH URL must not contain an empty user")
	}
	if _, present := parsed.User.Password(); present {
		return errors.New("must not contain a password")
	}
	return nil
}

func validSCPLikeRemote(remote string) bool {
	if strings.Contains(remote, "://") || strings.ContainsAny(remote, "?#") {
		return false
	}
	user, hostPath, found := strings.Cut(remote, "@")
	if !found {
		hostPath = user
	} else if !validSCPUser(user) || strings.Contains(hostPath, "@") {
		return false
	}
	return validSCPHostPath(hostPath)
}

func validSCPUser(user string) bool {
	return user != "" && !strings.ContainsAny(user, "/:@")
}

func validSCPHostPath(hostPath string) bool {
	if strings.HasPrefix(hostPath, "[") {
		return validBracketedSCPHostPath(hostPath)
	}
	host, path, found := strings.Cut(hostPath, ":")
	return found && host != "" && path != "" && !strings.ContainsAny(host, "/@")
}

func validBracketedSCPHostPath(hostPath string) bool {
	closing := strings.IndexByte(hostPath, ']')
	return closing > 1 && len(hostPath) > closing+2 && hostPath[closing+1] == ':' && hostPath[closing+2:] != ""
}

func decodeSingleEntry(contents []byte) (Entry, error) {
	if !utf8.Valid(contents) {
		return Entry{}, errors.New("catalog entry must contain valid UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	entry, err := decodeEntry(decoder)
	if err != nil {
		return Entry{}, err
	}
	if err = requireEOF(decoder, "catalog entry"); err != nil {
		return Entry{}, err
	}
	return entry, nil
}

func decodeSingleObject(contents []byte, description string) (map[string]json.RawMessage, error) {
	if !utf8.Valid(contents) {
		return nil, fmt.Errorf("%s must contain valid UTF-8", description)
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	if err := requireJSONDelimiter(decoder, '{', description+" must be a JSON object"); err != nil {
		return nil, err
	}
	values, err := decodeFields(decoder)
	if err != nil {
		return nil, err
	}
	if err = requireJSONDelimiter(decoder, '}', description+" object is not closed"); err != nil {
		return nil, err
	}
	if err = requireEOF(decoder, description); err != nil {
		return nil, err
	}
	return values, nil
}

func decodeEntry(decoder *json.Decoder) (Entry, error) {
	if err := requireJSONDelimiter(decoder, '{', "catalog entry must be a JSON object"); err != nil {
		return Entry{}, err
	}
	values, err := decodeFields(decoder)
	if err != nil {
		return Entry{}, err
	}
	if err = requireJSONDelimiter(decoder, '}', "catalog entry object is not closed"); err != nil {
		return Entry{}, err
	}
	return entryFromValues(values)
}

func decodeFields(decoder *json.Decoder) (map[string]json.RawMessage, error) {
	values := map[string]json.RawMessage{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		field, ok := token.(string)
		if !ok {
			return nil, errors.New("catalog field name must be a string")
		}
		if _, duplicate := values[field]; duplicate {
			return nil, fmt.Errorf("duplicate catalog field %q", field)
		}
		var value json.RawMessage
		if err = decoder.Decode(&value); err != nil {
			return nil, fmt.Errorf("decode catalog field %q: %w", field, err)
		}
		values[field] = value
	}
	return values, nil
}

func entryFromValues(values map[string]json.RawMessage) (Entry, error) {
	for _, field := range []string{"id", "display_name", "canonical_url"} {
		if _, present := values[field]; !present {
			return Entry{}, fmt.Errorf("catalog entry is missing %q", field)
		}
	}
	entry := Entry{Additional: map[string]json.RawMessage{}}
	for field, value := range values {
		var target *string
		switch field {
		case "id":
			target = &entry.ID
		case "display_name":
			target = &entry.DisplayName
		case "canonical_url":
			target = &entry.CanonicalURL
		default:
			entry.Additional[field] = append(json.RawMessage(nil), value...)
			continue
		}
		if err := json.Unmarshal(value, target); err != nil {
			return Entry{}, fmt.Errorf("catalog field %q must be a string", field)
		}
	}
	if len(entry.Additional) == 0 {
		entry.Additional = nil
	}
	if err := entry.Validate(); err != nil {
		return Entry{}, fmt.Errorf("project %q: %w", entry.ID, err)
	}
	return entry, nil
}

func isRequiredField(field string) bool {
	return field == "id" || field == "display_name" || field == "canonical_url"
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

func requireEOF(decoder *json.Decoder, description string) error {
	token, err := decoder.Token()
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("unexpected JSON value %v after %s", token, description)
}
