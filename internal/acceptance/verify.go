package acceptance

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/LevitateOS/soda-os/internal/process"
	"github.com/LevitateOS/soda-os/internal/strictjson"
)

const acceptanceSigner = "https://github.com/LevitateOS/soda-os/.github/workflows/native-acceptance-evidence.yml@refs/heads/main"
const acceptanceIssuer = "https://token.actions.githubusercontent.com"

// VerifySignedRecord is the release consumer. Trust is fixed by the maintained
// signing workflow, never taken from the document being verified or a CLI flag.
// It verifies source-level acceptance; it does not claim CI's rebuilt bytes ran.
func VerifySignedRecord(ctx context.Context, path, revision string, runner process.Runner) error {
	if err := requireRegularFile(path); err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	var record AcceptanceRecord
	if err = strictjson.Decode(file, &record); err != nil {
		return fmt.Errorf("read acceptance record: %w", err)
	}
	if err = record.qualify(revision); err != nil {
		return err
	}
	bundle := path + ".sigstore.json"
	if err = requireRegularFile(bundle); err != nil {
		return err
	}
	options := RecordOptions{Output: path, ApprovedSigner: acceptanceSigner, OIDCIssuer: acceptanceIssuer}
	return runner.Run(ctx, cosignVerifyCommand(options, bundle))
}

func (record AcceptanceRecord) qualify(revision string) error {
	if record.SchemaVersion != 2 || !gitRevision(revision) || record.SourceRevision != revision || record.SuiteRevision != revision {
		return errors.New("acceptance must use schema 2 and the exact release source and suite revision")
	}
	if record.ApprovedSigner != acceptanceSigner || len(record.Architectures) != 2 {
		return errors.New("acceptance requires the approved signer and exactly two sibling runs")
	}
	if err := qualifySiblingRuns(record.Architectures, revision); err != nil {
		return err
	}
	completed, err := time.Parse(time.RFC3339, record.CompletedAt)
	if err != nil || !completed.Equal(latestCompletion(record.Architectures)) {
		return errors.New("acceptance completion time must be the latest sibling completion")
	}
	return nil
}

func qualifySiblingRuns(runs []RunSummary, revision string) error {
	if len(runs) != 2 {
		return errors.New("acceptance requires exactly two sibling runs")
	}
	seen := map[string]bool{}
	for _, run := range runs {
		if err := run.Qualify(); err != nil {
			return fmt.Errorf("qualify %s: %w", run.Architecture, err)
		}
		if seen[run.Architecture] || run.SourceRevision != revision || run.SuiteRevision != revision {
			return errors.New("distinct sibling source and suite revisions must equal the expected workflow revision")
		}
		seen[run.Architecture] = true
	}
	return nil
}

// Run timestamps have already been validated. Compare instants, not RFC3339
// strings: different UTC offsets need not sort in chronological order.
func latestCompletion(runs []RunSummary) time.Time {
	var latest time.Time
	for _, run := range runs {
		completed, _ := time.Parse(time.RFC3339, run.CompletedAt)
		if completed.After(latest) {
			latest = completed
		}
	}
	return latest
}
