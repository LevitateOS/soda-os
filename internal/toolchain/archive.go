package toolchain

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func checksumLine(contents, filename string) (string, error) {
	for _, line := range strings.Split(contents, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.TrimPrefix(fields[1], "*") == filename {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("checksum for %s not found", filename)
}
func findArtifact(items []artifact, tool string) *artifact {
	for index := range items {
		if items[index].tool == tool {
			return &items[index]
		}
	}
	return nil
}
func artifactBin(item artifact, destination string) string {
	if item.kind == executable {
		return filepath.Join(destination, "cargo", "bin")
	}
	return filepath.Join(destination, "bin")
}

func extractTarGz(payload []byte, destination string) error {
	reader, err := gzip.NewReader(bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer reader.Close()
	return extractTar(reader, destination)
}
func extractTar(reader io.Reader, destination string) error {
	archive := tar.NewReader(reader)
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := extractTarEntry(archive, header, destination); err != nil {
			return err
		}
	}
}

func extractTarEntry(archive *tar.Reader, header *tar.Header, destination string) error {
	target, include, err := tarTarget(destination, header.Name)
	if err != nil || !include {
		return err
	}
	return writeTarEntry(archive, header, target)
}

func tarTarget(destination, name string) (string, bool, error) {
	parts := strings.Split(filepath.Clean(name), string(os.PathSeparator))
	if len(parts) < 2 {
		return "", false, nil
	}
	target := filepath.Join(destination, filepath.Join(parts[1:]...))
	if !within(destination, target) {
		return "", false, errors.New("archive path escapes destination")
	}
	return target, true, nil
}
func writeTarEntry(archive *tar.Reader, header *tar.Header, target string) error {
	switch header.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, os.FileMode(header.Mode))
	case tar.TypeReg:
		return writeArchiveFile(archive, target, os.FileMode(header.Mode))
	case tar.TypeSymlink:
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.Symlink(header.Linkname, target)
	default:
		return nil
	}
}
func writeArchiveFile(source io.Reader, target string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, source)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
func extractTarXz(ctx context.Context, payload []byte, destination string) error {
	temporary, err := os.CreateTemp("", "soda-toolchain-*.tar.xz")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if _, err = temporary.Write(payload); err != nil {
		return err
	}
	if err = temporary.Close(); err != nil {
		return err
	}
	output, err := exec.CommandContext(ctx, "tar", "-xJf", name, "-C", destination, "--strip-components=1").CombinedOutput()
	if err != nil {
		return fmt.Errorf("extract xz archive: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
func extractZip(payload []byte, destination string) error {
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return err
	}
	for _, entry := range reader.File {
		if err := writeZipEntry(entry, destination); err != nil {
			return err
		}
	}
	return nil
}
func writeZipEntry(entry *zip.File, destination string) error {
	target := filepath.Join(destination, entry.Name)
	if !within(destination, target) {
		return errors.New("archive path escapes destination")
	}
	if entry.FileInfo().IsDir() {
		return os.MkdirAll(target, entry.Mode())
	}
	source, err := entry.Open()
	if err != nil {
		return err
	}
	defer source.Close()
	return writeArchiveFile(source, target, entry.Mode())
}
func within(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}
func normalizeBinary(root, name string) error {
	source, err := findFile(root, name)
	if err != nil {
		return err
	}
	bin := filepath.Join(root, "bin")
	if err = os.MkdirAll(bin, 0o755); err != nil {
		return err
	}
	target := filepath.Join(bin, name)
	if source != target {
		contents, readErr := os.ReadFile(source)
		if readErr != nil {
			return readErr
		}
		if err = os.WriteFile(target, contents, 0o755); err != nil {
			return err
		}
	}
	return os.Chmod(target, 0o755)
}
func findFile(root, name string) (string, error) {
	var candidates []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && entry.Name() == name {
			candidates = append(candidates, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("downloaded archive does not contain %s", name)
	}
	sort.Strings(candidates)
	return candidates[0], nil
}
func firstPythonBin(root string) (string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		bin := filepath.Join(root, entry.Name(), "bin")
		if exists(filepath.Join(bin, "python")) || exists(filepath.Join(bin, "python3")) {
			return bin, nil
		}
	}
	return "", errors.New("uv did not install a Python interpreter")
}
func exists(path string) bool { _, err := os.Stat(path); return err == nil }
func makeReadable(root, path string) error {
	if err := filepath.Walk(path, readableMode); err != nil {
		return err
	}
	return makeAncestorsTraversable(root, path)
}

func readableMode(current string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	mode := os.FileMode(0o644)
	if info.IsDir() || info.Mode()&0o111 != 0 {
		mode = 0o755
	}
	return os.Chmod(current, mode)
}

func makeAncestorsTraversable(root, path string) error {
	for current := filepath.Dir(path); within(root, current); current = filepath.Dir(current) {
		if err := os.Chmod(current, 0o755); err != nil {
			return err
		}
		if filepath.Clean(current) == filepath.Clean(root) {
			return nil
		}
	}
	return nil
}
