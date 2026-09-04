package acceptance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/LevitateOS/soda-os/internal/build/release"
	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/LevitateOS/soda-os/internal/process"
)

type AcceptanceRecord struct {
	SchemaVersion  uint32       `json:"schema_version"`
	SourceRevision string       `json:"source_revision"`
	SuiteRevision  string       `json:"acceptance_suite_revision"`
	Architectures  []RunSummary `json:"architectures"`
	CompletedAt    string       `json:"completed_at"`
	ApprovedSigner string       `json:"approved_signer"`
}

type RecordOptions struct {
	X86Summary         string
	ARM64Summary       string
	ARM64ReleaseRecord string
	ARM64Spec          config.DistroSpec
	ExpectedRevision   string
	Output             string
	ApprovedSigner     string
	OIDCIssuer         string
}

type RecordResult struct {
	RecordPath string
	BundlePath string
}

func CreateSignedRecord(ctx context.Context, options RecordOptions, runner process.Runner) (RecordResult, error) {
	if err := options.validate(); err != nil {
		return RecordResult{}, err
	}
	runs, err := readSiblingSummaries(options.X86Summary, options.ARM64Summary)
	if err != nil {
		return RecordResult{}, err
	}
	if err = validateRecordInputs(runs, options); err != nil {
		return RecordResult{}, err
	}
	record := combinedRecord(runs, options.ApprovedSigner)
	if err = writeNewJSON(options.Output, record); err != nil {
		return RecordResult{}, err
	}
	bundle := options.Output + ".sigstore.json"
	if err = runner.Run(ctx, cosignSignCommand(options.Output, bundle)); err != nil {
		return RecordResult{}, fmt.Errorf("sign acceptance record: %w", err)
	}
	if err = runner.Run(ctx, cosignVerifyCommand(options, bundle)); err != nil {
		return RecordResult{}, fmt.Errorf("verify acceptance record signature: %w", err)
	}
	return RecordResult{RecordPath: options.Output, BundlePath: bundle}, nil
}

func (options RecordOptions) validate() error {
	if options.Output == "" || options.ApprovedSigner == "" || options.OIDCIssuer == "" || options.ARM64ReleaseRecord == "" {
		return errors.New("record output, AArch64 release record, approved signer, and OIDC issuer are required")
	}
	if !gitRevision(options.ExpectedRevision) {
		return errors.New("expected source revision must be a full Git SHA")
	}
	return nil
}

func validateRecordInputs(runs []RunSummary, options RecordOptions) error {
	for _, run := range runs {
		if run.SourceRevision != options.ExpectedRevision || run.SuiteRevision != options.ExpectedRevision {
			return errors.New("sibling source and suite revisions must equal the expected workflow revision")
		}
	}
	armRecord, err := release.ValidateStrictRecord(options.ARM64ReleaseRecord, options.ARM64Spec, options.ExpectedRevision)
	if err != nil {
		return fmt.Errorf("validate AArch64 release record: %w", err)
	}
	if armRecord.SodaImageReference != release.Repository+"@"+runs[1].CandidateDigest {
		return errors.New("AArch64 release record image digest differs from the AArch64 run summary")
	}
	return nil
}

func readSiblingSummaries(x86Path, armPath string) ([]RunSummary, error) {
	x86, err := readRunSummary(x86Path)
	if err != nil {
		return nil, fmt.Errorf("read x86-64 run summary: %w", err)
	}
	arm, err := readRunSummary(armPath)
	if err != nil {
		return nil, fmt.Errorf("read AArch64 run summary: %w", err)
	}
	if x86.Architecture != "x86_64" || arm.Architecture != "aarch64" {
		return nil, errors.New("summary paths must name x86_64 and aarch64 runs respectively")
	}
	if x86.SourceRevision != arm.SourceRevision || x86.SuiteRevision != arm.SuiteRevision {
		return nil, errors.New("sibling runs must name the same source and suite revisions")
	}
	return []RunSummary{x86, arm}, nil
}

func readRunSummary(path string) (RunSummary, error) {
	if err := requireRegularFile(path); err != nil {
		return RunSummary{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return RunSummary{}, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var summary RunSummary
	if err = decoder.Decode(&summary); err != nil {
		return RunSummary{}, err
	}
	if err = requireJSONEOF(decoder); err != nil {
		return RunSummary{}, err
	}
	return summary, summary.Validate()
}

func requireJSONEOF(decoder *json.Decoder) error {
	var trailing any
	err := decoder.Decode(&trailing)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("decode trailing JSON: %w", err)
	}
	return errors.New("record contains trailing JSON")
}

func combinedRecord(runs []RunSummary, signer string) AcceptanceRecord {
	sort.Slice(runs, func(left, right int) bool { return runs[left].Architecture < runs[right].Architecture })
	completed := runs[0].CompletedAt
	if runs[1].CompletedAt > completed {
		completed = runs[1].CompletedAt
	}
	return AcceptanceRecord{
		SchemaVersion:  1,
		SourceRevision: runs[0].SourceRevision,
		SuiteRevision:  runs[0].SuiteRevision,
		Architectures:  runs,
		CompletedAt:    completed,
		ApprovedSigner: signer,
	}
}

func writeNewJSON(path string, value any) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create acceptance record: %w", err)
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encodeErr := encoder.Encode(value)
	closeErr := file.Close()
	if encodeErr != nil {
		return encodeErr
	}
	return closeErr
}

func cosignSignCommand(record, bundle string) process.Command {
	return process.Command{Name: "cosign", Args: []string{"sign-blob", "--yes", "--bundle", bundle, record}}
}

func cosignVerifyCommand(options RecordOptions, bundle string) process.Command {
	return process.Command{Name: "cosign", Args: []string{
		"verify-blob", "--bundle", bundle,
		"--certificate-identity", options.ApprovedSigner,
		"--certificate-oidc-issuer", options.OIDCIssuer,
		options.Output,
	}}
}

func SummaryTime(now time.Time) string {
	return now.UTC().Format(time.RFC3339)
}
