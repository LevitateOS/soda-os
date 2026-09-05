package updates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/LevitateOS/soda-os/internal/process"
)

func (r *Releases) verifyBlob(ctx context.Context, path string) error {
	args := []string{"verify-blob", "--bundle", path + ".sigstore.json", "--certificate-identity", signer, "--certificate-oidc-issuer", issuer, path}
	if err := r.runner.Run(ctx, process.Command{Name: "/usr/bin/cosign", Args: args}); err != nil {
		return fmt.Errorf("verify Soda release record: %w", err)
	}
	return nil
}

func (r *Releases) verifyImage(ctx context.Context, selected Release, platform string) error {
	for _, operation := range []string{"verify", "verify-attestation"} {
		args := []string{operation, "--certificate-identity", signer, "--certificate-oidc-issuer", issuer}
		if operation == "verify-attestation" {
			args = append(args, "--type", "slsaprovenance")
		}
		args = append(args, selected.Reference)
		if err := r.runner.Run(ctx, process.Command{Name: "/usr/bin/cosign", Args: args}); err != nil {
			return fmt.Errorf("verify exact Soda image: %w", err)
		}
	}
	output, err := r.runner.Output(ctx, process.Command{Name: "/usr/bin/skopeo", Args: []string{"inspect", "--no-creds", "docker://" + selected.Reference}})
	if err != nil {
		return fmt.Errorf("inspect exact Soda image anonymously: %w", err)
	}
	return validateImage(output, selected, platform)
}

func validateImage(output string, selected Release, platform string) error {
	var image struct {
		Digest       string
		Os           string
		Architecture string
		Labels       map[string]string
	}
	if err := json.Unmarshal([]byte(output), &image); err != nil {
		return fmt.Errorf("decode OCI metadata: %w", err)
	}
	if repository+"@"+image.Digest != selected.Reference || image.Os+"/"+image.Architecture != platform {
		return errors.New("remote image digest or platform differs from the signed Soda release")
	}
	if image.Labels["org.opencontainers.image.version"] != selected.Version || image.Labels["org.opencontainers.image.revision"] != selected.Revision {
		return errors.New("remote image version or source revision differs from the signed Soda release")
	}
	return nil
}

// CompareStableVersions compares published stable versions numerically, without
// integer overflow. Development versions require an explicit administrator
// decision and are not automatically treated as an older stable installation.
func CompareStableVersions(installed, available string) (int, error) {
	if !stableVersion.MatchString(installed) || !stableVersion.MatchString(available) {
		return 0, errors.New("automatic update selection requires stable MAJOR.MINOR.PATCH versions; use native bootc for development images")
	}
	left, right := strings.Split(installed, "."), strings.Split(available, ".")
	for index := range left {
		if len(left[index]) < len(right[index]) {
			return -1, nil
		}
		if len(left[index]) > len(right[index]) {
			return 1, nil
		}
		if comparison := strings.Compare(left[index], right[index]); comparison != 0 {
			return comparison, nil
		}
	}
	return 0, nil
}
