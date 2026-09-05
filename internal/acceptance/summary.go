package acceptance

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"
)

// Qualification requires observations, not merely completion of the runner.
// tests/acceptance/README.md maps each name to its evidence and limitations.
var requiredChecks = []string{
	"iso-first-boot-defaults",
	"qcow2-cloud-init-local",
	"local-forwarded-access",
	"tailnet-access",
	"installed-onboarding-observations",
	"trusted-lan-access",
	"public-ingress-rejection",
	"workspace-boundaries-and-git-keys",
	"ssh-transports",
	"development-server-access",
	"native-mise-ownership",
	"workspace-removal",
	"cockpit-auth-and-independent-roles",
	"external-ssh-repository",
	"project-removal",
	"human-removal-preserves-forgejo",
	"update-and-fallback",
	"packaged-boundaries",
	"runner-completion",
	"evidence-and-cleanup",
}

type RunSummary struct {
	SchemaVersion   uint32            `json:"schema_version"`
	Architecture    string            `json:"architecture"`
	Platform        string            `json:"platform"`
	SourceRevision  string            `json:"source_revision"`
	SuiteRevision   string            `json:"suite_revision"`
	CandidateDigest string            `json:"candidate_digest"`
	FallbackDigest  string            `json:"fallback_digest"`
	Scenarios       map[string]string `json:"scenarios"`
	CompletedAt     string            `json:"completed_at"`
}

// Validate accepts partial reports. Missing checks are not established.
func (summary RunSummary) Validate() error {
	if summary.SchemaVersion != 2 {
		return fmt.Errorf("run summary schema is %d, expected 2", summary.SchemaVersion)
	}
	if summary.Architecture != "x86_64" && summary.Architecture != "aarch64" {
		return errors.New("run summary architecture must be x86_64 or aarch64")
	}
	expectedPlatform := map[string]string{"x86_64": "linux/amd64", "aarch64": "linux/arm64"}[summary.Architecture]
	if summary.Platform != expectedPlatform {
		return errors.New("run summary platform does not match its architecture")
	}
	if !gitRevision(summary.SourceRevision) || !gitRevision(summary.SuiteRevision) {
		return errors.New("run summary revisions must be full Git SHAs")
	}
	if !validDigestPair(summary.CandidateDigest, summary.FallbackDigest) {
		return errors.New("run summary image digests must be exact sha256 digests and must differ")
	}
	if _, err := time.Parse(time.RFC3339, summary.CompletedAt); err != nil {
		return errors.New("run summary completion time must be RFC3339")
	}
	return validateScenarioResults(summary.Scenarios)
}

func validDigestPair(candidate, fallback string) bool {
	return digest(candidate) && digest(fallback) && candidate != fallback
}

func gitRevision(value string) bool {
	if len(value) != 40 {
		return false
	}
	for _, character := range value {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return false
		}
	}
	return true
}

func validateScenarioResults(results map[string]string) error {
	for name, result := range results {
		if !slices.Contains(requiredChecks, name) {
			return fmt.Errorf("unknown acceptance check %s", name)
		}
		if result != "pass" {
			return fmt.Errorf("check %s has invalid result %q; omit checks not established", name, result)
		}
	}
	return nil
}

// Qualify is the signing boundary; structural validity alone is insufficient.
func (summary RunSummary) Qualify() error {
	if err := summary.Validate(); err != nil {
		return err
	}
	var missing []string
	for _, name := range requiredChecks {
		if summary.Scenarios[name] != "pass" {
			missing = append(missing, name)
		}
	}
	if len(missing) != 0 {
		return fmt.Errorf("incomplete qualification; missing evidence for: %s", strings.Join(missing, ", "))
	}
	return nil
}

func digest(value string) bool {
	return len(value) == 71 && exactReference("image@"+value)
}

func WriteRunSummary(path string, summary RunSummary) error {
	if err := summary.Validate(); err != nil {
		return err
	}
	contents, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	contents = append(contents, '\n')
	return os.WriteFile(path, contents, 0o600)
}

// checkResults records only successful observations at their call sites.
// It neither schedules checks nor infers coverage from other results.
type checkResults map[string]string

func (results checkResults) record(name string, err error) error {
	if err != nil {
		delete(results, name)
		return fmt.Errorf("check %s: %w", name, err)
	}
	results[name] = "pass"
	return nil
}
