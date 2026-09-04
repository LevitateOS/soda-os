package projects

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
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
	for field := range values {
		if catalogCredentialField(field) {
			return fmt.Errorf("catalog metadata must not contain credential field %q", field)
		}
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

func catalogCredentialField(field string) bool {
	switch strings.ToLower(field) {
	case "password", "forgejo_password", "git_password", "token", "tea_token", "gh_token", "credential", "credentials", "private_key":
		return true
	default:
		return false
	}
}

type AddExistingRequest = CatalogMutationRequest

type CreateForgejoRequest struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

type EditRequest = AddExistingRequest

type SetupRequest struct {
	ID              string   `json:"id"`
	ForgejoPassword string   `json:"forgejo_password"`
	WorkspaceTools  []string `json:"workspace_tools"`
	ProjectTools    []string `json:"project_tools"`
}

type ToolRequest struct {
	ID    string   `json:"id"`
	Scope string   `json:"scope"`
	Tools []string `json:"tools"`
}

type ProjectRequest struct {
	ID string `json:"id"`
}

type DeleteHumanRequest struct {
	Username string `json:"username"`
}

type AddPersonRequest struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	AuthorizedKey string `json:"authorized_key"`
}

type ProjectView struct {
	CatalogEntry
	WorkspaceUsername string `json:"workspace_username"`
	WorkspaceReady    bool   `json:"workspace_ready"`
}

func (view ProjectView) MarshalJSON() ([]byte, error) {
	object := view.CatalogEntry.jsonObject()
	metadata := view.CatalogEntry.Additional
	if metadata == nil {
		metadata = map[string]json.RawMessage{}
	}
	object["catalog_metadata"], _ = json.Marshal(metadata)
	object["workspace_username"], _ = json.Marshal(view.WorkspaceUsername)
	object["workspace_ready"], _ = json.Marshal(view.WorkspaceReady)
	return json.Marshal(object)
}

type CurrentUserView struct {
	Username      string `json:"username"`
	Administrator bool   `json:"administrator"`
}

type ListResponse struct {
	Projects    []ProjectView   `json:"projects"`
	CurrentUser CurrentUserView `json:"current_user"`
	ForgejoURL  string          `json:"forgejo_url"`
	SSHHost     string          `json:"ssh_host"`
}

type MutationResponse struct {
	OK                 bool         `json:"ok"`
	Project            *ProjectView `json:"project,omitempty"`
	WorkspaceUsername  string       `json:"workspace_username,omitempty"`
	WorkspacePublicKey string       `json:"workspace_public_key,omitempty"`
}

type HelperCatalogRequest = CatalogMutationRequest

type HelperWorkspaceRequest struct {
	ID             string   `json:"id"`
	CanonicalURL   string   `json:"canonical_url"`
	WorkspaceTools []string `json:"workspace_tools"`
	ProjectTools   []string `json:"project_tools"`
}

type HelperToolRequest = ToolRequest

type HelperHumanRequest struct {
	Username string `json:"username"`
}

type HelperHumanCreateRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type HelperHumanPublishRequest struct {
	Username      string `json:"username"`
	AuthorizedKey string `json:"authorized_key"`
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
