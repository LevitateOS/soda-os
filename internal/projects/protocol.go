package projects

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

type EmptyRequest struct{}

type CatalogMutationRequest struct {
	CatalogEntry
}

func (request CatalogMutationRequest) MarshalJSON() ([]byte, error) {
	return json.Marshal(request.CatalogEntry.jsonObject())
}

func (request *CatalogMutationRequest) UnmarshalJSON(contents []byte) error {
	var values map[string]json.RawMessage
	if err := json.Unmarshal(contents, &values); err != nil {
		return err
	}
	entry, err := catalogEntryFromValues(values)
	if err != nil {
		return err
	}
	if entry.Additional == nil {
		entry.Additional = map[string]json.RawMessage{}
	}
	request.CatalogEntry = entry
	return nil
}

type AddExistingRequest = CatalogMutationRequest

type EditRequest struct {
	ID          string                     `json:"id"`
	DisplayName string                     `json:"display_name"`
	Additional  map[string]json.RawMessage `json:"-"`
}

func (request EditRequest) Validate() error {
	if err := validateCatalogIdentity(request.ID, request.DisplayName); err != nil {
		return err
	}
	return validateCatalogAdditionalFields(request.Additional)
}

func (request EditRequest) MarshalJSON() ([]byte, error) {
	object := make(map[string]json.RawMessage, len(request.Additional)+2)
	for field, value := range request.Additional {
		object[field] = append(json.RawMessage(nil), value...)
	}
	object["id"], _ = json.Marshal(request.ID)
	object["display_name"], _ = json.Marshal(request.DisplayName)
	return json.Marshal(object)
}

func (request *EditRequest) UnmarshalJSON(contents []byte) error {
	var values map[string]json.RawMessage
	if err := json.Unmarshal(contents, &values); err != nil {
		return err
	}
	decoded, err := editRequestFromValues(values)
	if err != nil {
		return err
	}
	*request = decoded
	return nil
}

func editRequestFromValues(values map[string]json.RawMessage) (EditRequest, error) {
	if _, present := values["canonical_url"]; present {
		return EditRequest{}, errors.New(`edit request must not include "canonical_url"; remove and re-add the project to replace its canonical URL`)
	}
	for _, field := range []string{"id", "display_name"} {
		if _, present := values[field]; !present {
			return EditRequest{}, fmt.Errorf("edit request is missing %q", field)
		}
	}
	request := EditRequest{Additional: map[string]json.RawMessage{}}
	for field, value := range values {
		if err := decodeEditField(&request, field, value); err != nil {
			return EditRequest{}, err
		}
	}
	return request, nil
}

func decodeEditField(request *EditRequest, field string, value json.RawMessage) error {
	var target *string
	switch field {
	case "id":
		target = &request.ID
	case "display_name":
		target = &request.DisplayName
	default:
		request.Additional[field] = append(json.RawMessage(nil), value...)
		return nil
	}
	if err := json.Unmarshal(value, target); err != nil {
		return fmt.Errorf("edit field %q must be a string", field)
	}
	return nil
}

func (request EditRequest) apply(current CatalogEntry) CatalogEntry {
	additional := request.Additional
	if additional == nil {
		additional = current.Additional
	}
	return CatalogEntry{
		ID:           current.ID,
		DisplayName:  request.DisplayName,
		CanonicalURL: current.CanonicalURL,
		Additional:   additional,
	}
}

type SetupRequest struct {
	ID string `json:"id"`
}

type ProjectRequest struct {
	ID string `json:"id"`
}

type DeleteHumanRequest struct {
	Username string `json:"username"`
}

type ProjectView struct {
	CatalogEntry
	WorkspaceUsername string `json:"workspace_username"`
	WorkspaceExists   bool   `json:"workspace_exists"`
}

func (view ProjectView) MarshalJSON() ([]byte, error) {
	object := map[string]json.RawMessage{}
	object["id"], _ = json.Marshal(view.ID)
	object["display_name"], _ = json.Marshal(view.DisplayName)
	object["canonical_url"], _ = json.Marshal(view.CanonicalURL)
	metadata := view.CatalogEntry.Additional
	if metadata == nil {
		metadata = map[string]json.RawMessage{}
	}
	object["catalog_metadata"], _ = json.Marshal(metadata)
	object["workspace_username"], _ = json.Marshal(view.WorkspaceUsername)
	object["workspace_exists"], _ = json.Marshal(view.WorkspaceExists)
	return json.Marshal(object)
}

type CurrentUserView struct {
	Username      string `json:"username"`
	Administrator bool   `json:"administrator"`
}

type ListResponse struct {
	Projects    []ProjectView   `json:"projects"`
	CurrentUser CurrentUserView `json:"current_user"`
}

type MutationResponse struct {
	OK                 bool         `json:"ok"`
	Project            *ProjectView `json:"project,omitempty"`
	WorkspaceUsername  string       `json:"workspace_username,omitempty"`
	WorkspacePublicKey string       `json:"workspace_public_key,omitempty"`
}

type HelperCatalogRequest = CatalogMutationRequest

type HelperEditRequest = EditRequest

type HelperWorkspaceRequest struct {
	ID           string `json:"id"`
	CanonicalURL string `json:"canonical_url"`
}

type HelperHumanRequest struct {
	Username string `json:"username"`
}

// DecodeRequest accepts exactly one flat JSON object, rejects duplicate and
// unknown fields, and never logs its contents.
func DecodeRequest(reader io.Reader, destination any) error {
	contents, err := io.ReadAll(io.LimitReader(reader, 1<<20+1))
	if err != nil {
		return fmt.Errorf("read request: %w", err)
	}
	if len(contents) > 1<<20 {
		return errors.New("request exceeds 1 MiB")
	}
	if !utf8.Valid(contents) {
		return errors.New("request must contain valid UTF-8")
	}
	object, err := decodeUniqueObject(contents)
	if err != nil {
		return err
	}
	normalized, err := json.Marshal(object)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(normalized))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(destination); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	return nil
}

func decodeUniqueObject(contents []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	if err := requireRequestObject(decoder); err != nil {
		return nil, err
	}
	object, err := decodeRequestFields(decoder)
	if err != nil {
		return nil, err
	}
	if err = finishRequestObject(decoder); err != nil {
		return nil, err
	}
	return object, nil
}

func requireRequestObject(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	delimiter, ok := token.(json.Delim)
	if !ok || delimiter != '{' {
		return errors.New("request must be one JSON object")
	}
	return nil
}

func decodeRequestFields(decoder *json.Decoder) (map[string]json.RawMessage, error) {
	object := map[string]json.RawMessage{}
	for decoder.More() {
		field, err := decodeRequestFieldName(decoder, object)
		if err != nil {
			return nil, err
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, fmt.Errorf("decode request field %q: %w", field, err)
		}
		object[field] = value
	}
	return object, nil
}

func decodeRequestFieldName(decoder *json.Decoder, object map[string]json.RawMessage) (string, error) {
	token, err := decoder.Token()
	if err != nil {
		return "", fmt.Errorf("decode request: %w", err)
	}
	field, ok := token.(string)
	if !ok {
		return "", errors.New("request field name must be a string")
	}
	if _, duplicate := object[field]; duplicate {
		return "", fmt.Errorf("duplicate request field %q", field)
	}
	return field, nil
}

func finishRequestObject(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	delimiter, ok := token.(json.Delim)
	if !ok || delimiter != '}' {
		return errors.New("request object is not closed")
	}
	token, err = decoder.Token()
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	return fmt.Errorf("unexpected JSON value %v after request", token)
}
