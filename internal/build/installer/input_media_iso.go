package installer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LevitateOS/soda-os/internal/process"
)

type installerInputWorkspace struct {
	root      string
	staging   string
	output    string
	published bool
}

func (b *InstallerInputBuilder) createMedia(ctx context.Context, outputPath string, files map[string][]byte) (resultErr error) {
	workspace, err := newInstallerInputWorkspace(outputPath, files)
	if err != nil {
		return err
	}
	defer workspace.cleanup(&resultErr)

	temporaryISO := filepath.Join(workspace.root, "installer-input.iso")
	if err = b.runInstallerInputXorriso(ctx, workspace.staging, temporaryISO); err != nil {
		return err
	}
	if err = b.verifyMedia(ctx, workspace.root, temporaryISO, files); err != nil {
		return err
	}
	if err = removeInstallerInputPlaintext(workspace.root, workspace.staging); err != nil {
		return err
	}
	if err = publishInstallerInputISO(temporaryISO, workspace.output); err != nil {
		return err
	}
	workspace.published = true
	return nil
}

func newInstallerInputWorkspace(outputPath string, files map[string][]byte) (*installerInputWorkspace, error) {
	outputAbsolute, err := filepath.Abs(outputPath)
	if err != nil {
		return nil, fmt.Errorf("resolve installer input output: %w", err)
	}
	root, staging, err := stageInstallerInputFiles(outputAbsolute, files)
	if err != nil {
		return nil, err
	}
	return &installerInputWorkspace{root: root, staging: staging, output: outputAbsolute}, nil
}

func (w *installerInputWorkspace) cleanup(resultErr *error) {
	cleanupErr := os.RemoveAll(w.root)
	if cleanupErr == nil {
		return
	}
	if w.published {
		_ = os.Remove(w.output)
	}
	cleanupErr = fmt.Errorf("remove private installer input workspace: %w", cleanupErr)
	if *resultErr == nil {
		*resultErr = cleanupErr
		return
	}
	*resultErr = fmt.Errorf("%v; %w", *resultErr, cleanupErr)
}

func removeInstallerInputPlaintext(work, staging string) error {
	for _, path := range []string{staging, filepath.Join(work, "verified")} {
		if err := removeInstallerInputPlaintextPath(path); err != nil {
			return err
		}
	}
	return nil
}

func removeInstallerInputPlaintextPath(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove staged installer input plaintext: %w", err)
	}
	_, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err == nil {
		return errors.New("staged installer input plaintext remains")
	}
	return fmt.Errorf("verify staged installer input plaintext removal: %w", err)
}

func stageInstallerInputFiles(outputAbsolute string, files map[string][]byte) (string, string, error) {
	work, err := os.MkdirTemp(filepath.Dir(outputAbsolute), ".soda-installer-input-*")
	if err != nil {
		return "", "", fmt.Errorf("create private installer input workspace: %w", err)
	}
	staging, err := createInstallerInputStaging(work)
	if err != nil {
		_ = os.RemoveAll(work)
		return "", "", err
	}
	if err = writeInstallerInputFiles(staging, files); err != nil {
		_ = os.RemoveAll(work)
		return "", "", err
	}
	return work, staging, nil
}

func createInstallerInputStaging(work string) (string, error) {
	staging := filepath.Join(work, "input")
	if err := os.Mkdir(staging, 0o700); err != nil {
		return "", fmt.Errorf("create installer input staging directory: %w", err)
	}
	if err := os.Mkdir(filepath.Join(staging, "soda"), 0o700); err != nil {
		return "", fmt.Errorf("create installer input data directory: %w", err)
	}
	return staging, nil
}

func writeInstallerInputFiles(staging string, files map[string][]byte) error {
	for _, name := range installerInputPaths() {
		if err := os.WriteFile(filepath.Join(staging, name), files[name], 0o600); err != nil {
			return fmt.Errorf("stage installer input data: %w", err)
		}
	}
	return nil
}

func (b *InstallerInputBuilder) runInstallerInputXorriso(ctx context.Context, staging, temporaryISO string) error {
	args := []string{"-as", "mkisofs", "-quiet", "-V", "OEMDRV", "-graft-points", "-o", temporaryISO}
	for _, name := range installerInputPaths() {
		args = append(args, name+"="+name)
	}
	if err := b.runner.Run(ctx, process.Command{Dir: staging, Name: "xorriso", Args: args}); err != nil {
		return fmt.Errorf("create installer input ISO: %w", err)
	}
	return nil
}

func publishInstallerInputISO(temporaryISO, outputAbsolute string) error {
	if err := prepareInstallerInputISO(temporaryISO); err != nil {
		return err
	}
	if err := linkInstallerInputISO(temporaryISO, outputAbsolute); err != nil {
		return err
	}
	if err := syncInstallerInputOutputDirectory(filepath.Dir(outputAbsolute)); err != nil {
		_ = os.Remove(outputAbsolute)
		return err
	}
	return nil
}

func prepareInstallerInputISO(path string) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("xorriso did not create a regular installer input ISO")
	}
	if err = os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("protect installer input ISO: %w", err)
	}
	return syncInstallerInputISO(path)
}

func syncInstallerInputISO(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open installer input ISO: %w", err)
	}
	if err = file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync installer input ISO: %w", err)
	}
	if err = file.Close(); err != nil {
		return fmt.Errorf("close installer input ISO: %w", err)
	}
	return nil
}

func linkInstallerInputISO(temporaryISO, outputAbsolute string) error {
	if err := os.Link(temporaryISO, outputAbsolute); err != nil {
		if errors.Is(err, os.ErrExist) {
			return errors.New("installer input output already exists")
		}
		return fmt.Errorf("publish installer input ISO: %w", err)
	}
	return nil
}

func syncInstallerInputOutputDirectory(path string) error {
	parent, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open installer input output directory: %w", err)
	}
	if err = parent.Sync(); err != nil {
		_ = parent.Close()
		return fmt.Errorf("sync installer input output directory: %w", err)
	}
	if err = parent.Close(); err != nil {
		return fmt.Errorf("close installer input output directory: %w", err)
	}
	return nil
}

func (b *InstallerInputBuilder) verifyMedia(ctx context.Context, work, isoPath string, expected map[string][]byte) error {
	extracted, err := b.extractInstallerInputMedia(ctx, work, isoPath)
	if err != nil {
		return err
	}
	actual, err := readExtractedInstallerInput(extracted, len(expected))
	if err != nil {
		return err
	}
	return compareInstallerInputContents(actual, expected)
}

func (b *InstallerInputBuilder) extractInstallerInputMedia(ctx context.Context, work, isoPath string) (string, error) {
	extracted := filepath.Join(work, "verified")
	if err := os.Mkdir(extracted, 0o700); err != nil {
		return "", fmt.Errorf("create installer input verification directory: %w", err)
	}
	command := process.Command{
		Name: "xorriso",
		Args: []string{"-osirrox", "on", "-indev", isoPath, "-extract", "/", extracted},
	}
	if err := b.runner.Run(ctx, command); err != nil {
		return "", fmt.Errorf("inspect installer input ISO: %w", err)
	}
	return extracted, nil
}

type installerInputCollector struct {
	root  string
	files map[string][]byte
}

func readExtractedInstallerInput(root string, capacity int) (map[string][]byte, error) {
	collector := installerInputCollector{root: root, files: make(map[string][]byte, capacity)}
	if err := filepath.WalkDir(root, collector.visit); err != nil {
		return nil, fmt.Errorf("verify installer input ISO contents: %w", err)
	}
	return collector.files, nil
}

func (c installerInputCollector) visit(path string, entry os.DirEntry, walkErr error) error {
	if walkErr != nil {
		return walkErr
	}
	relative, err := filepath.Rel(c.root, path)
	if err != nil || relative == "." {
		return err
	}
	relative = filepath.ToSlash(relative)
	if entry.IsDir() {
		return validateInstallerInputDirectory(relative)
	}
	return c.collectFile(path, relative, entry)
}

func validateInstallerInputDirectory(relative string) error {
	if relative != "soda" {
		return fmt.Errorf("installer input ISO contains unexpected directory %s", relative)
	}
	return nil
}

func (c installerInputCollector) collectFile(path, relative string, entry os.DirEntry) error {
	info, err := entry.Info()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("installer input ISO contains non-regular path %s", relative)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	c.files[relative] = contents
	return nil
}

func compareInstallerInputContents(actual, expected map[string][]byte) error {
	if len(actual) != len(expected) {
		return errors.New("installer input ISO does not contain exactly the required files")
	}
	for _, name := range installerInputPaths() {
		contents, present := actual[name]
		if !present || !bytes.Equal(contents, expected[name]) {
			return fmt.Errorf("installer input ISO contains unexpected data for %s", name)
		}
	}
	return nil
}

func installerInputPaths() []string {
	return []string{
		"ks.cfg",
		"soda/administrator-username",
		"soda/administrator-password",
		"soda/administrator-authorized-key",
		"soda/tailscale-auth-key",
	}
}
