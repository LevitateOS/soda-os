package acceptance

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

var RequiredScenarios = []string{
	"installation-first-boot",
	"qcow2-cloud-init-lan",
	"lan-and-tailscale-access",
	"public-ingress-rejection",
	"ssh-cockpit-projects-forgejo",
	"native-mise-ownership",
	"identity-and-deletion",
	"update-and-fallback",
	"forbidden-state-absence",
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

func (summary RunSummary) Validate() error {
	if summary.SchemaVersion != 1 {
		return fmt.Errorf("run summary schema is %d, expected 1", summary.SchemaVersion)
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
	if len(results) != len(RequiredScenarios) {
		return errors.New("run summary has an incomplete scenario set")
	}
	for _, scenario := range RequiredScenarios {
		if results[scenario] != "pass" {
			return fmt.Errorf("scenario %s did not pass", scenario)
		}
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

func passedScenarios() map[string]string {
	results := make(map[string]string, len(RequiredScenarios))
	names := append([]string(nil), RequiredScenarios...)
	sort.Strings(names)
	for _, name := range names {
		results[name] = "pass"
	}
	return results
}
