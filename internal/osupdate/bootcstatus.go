package osupdate

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
)

type bootcStatusDocument struct {
	Status struct {
		Booted   *bootcDeployment `json:"booted"`
		Staged   *bootcDeployment `json:"staged"`
		ReadOnly bool             `json:"readOnly"`
	} `json:"status"`
}
type bootcDeployment struct {
	Image struct {
		Image struct {
			Image     string `json:"image"`
			Transport string `json:"transport"`
		} `json:"image"`
		Version      string `json:"version"`
		ImageDigest  string `json:"imageDigest"`
		Architecture string `json:"architecture"`
	} `json:"image"`
	Incompatible bool `json:"incompatible"`
	DownloadOnly bool `json:"downloadOnly"`
}

func parseBootcStatus(contents []byte, platform platformContract) (Status, error) {
	var document bootcStatusDocument
	if err := json.Unmarshal(contents, &document); err != nil {
		return Status{}, fmt.Errorf("decode bootc status: %w", err)
	}
	if document.Status.Booted == nil {
		return Status{}, errors.New("host has no booted bootc deployment")
	}
	booted, err := deployment(document.Status.Booted, platform)
	if err != nil {
		return Status{}, fmt.Errorf("decode booted deployment: %w", err)
	}
	result := Status{Booted: &booted, ReadOnly: document.Status.ReadOnly}
	if document.Status.Staged == nil {
		return result, nil
	}
	staged, err := deployment(document.Status.Staged, platform)
	if err != nil {
		return Status{}, fmt.Errorf("decode staged deployment: %w", err)
	}
	result.Staged = &staged
	return result, nil
}
func deployment(value *bootcDeployment, platform platformContract) (Deployment, error) {
	digest := strings.TrimSpace(value.Image.ImageDigest)
	if !validSHA256(digest) {
		return Deployment{}, errors.New("deployment has no valid image digest")
	}
	ref, err := name.ParseReference(value.Image.Image.Image)
	if err != nil || ref.Context().Name() != Repository {
		return Deployment{}, errors.New("deployment is not a Soda OS image")
	}
	if value.Image.Image.Transport != "registry" {
		return Deployment{}, errors.New("deployment is not registry-backed")
	}
	if value.Image.Architecture != platform.ociArchitecture {
		return Deployment{}, fmt.Errorf("deployment is not %s", platform.artifactArchitecture)
	}
	if value.Incompatible {
		return Deployment{}, errors.New("deployment is incompatible with bootc mutation")
	}
	return Deployment{ImageReference: Repository + "@" + digest, Version: value.Image.Version, Digest: digest, Architecture: value.Image.Architecture, Incompatible: value.Incompatible, DownloadOnly: value.DownloadOnly}, nil
}
func isSodaDigestReference(value string) bool {
	ref, err := name.NewDigest(value)
	return err == nil && ref.Context().Name() == Repository && validSHA256(ref.DigestStr())
}
func isDigestReference(value string) bool {
	ref, err := name.NewDigest(value)
	return err == nil && validSHA256(ref.DigestStr())
}
func validSHA256(value string) bool {
	return strings.HasPrefix(value, "sha256:") && len(value) == len("sha256:")+64 && hexadecimal(strings.TrimPrefix(value, "sha256:"))
}

func matchesDownloadedDeployment(deployment *Deployment, exactReference, architecture string) bool {
	return deployment != nil && deployment.ImageReference == exactReference && deployment.DownloadOnly && deployment.Architecture == architecture && !deployment.Incompatible
}
