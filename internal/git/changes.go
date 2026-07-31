package git

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const maxDiffFileSize = 5 << 20

var (
	ErrInvalidDiffPath = errors.New("invalid repository diff path")
	ErrDiffTooLarge    = errors.New("repository diff file is too large")
	ErrBinaryDiff      = errors.New("repository diff file is binary")
	ErrChangeNotFound  = errors.New("repository path has no changes")
)

type Change struct {
	Path         string `json:"path"`
	Status       string `json:"status"`
	PreviousPath string `json:"previous_path,omitempty"`
}

func (p Provider) ChangedFiles(ctx context.Context, repository string) ([]Change, error) {
	out, err := p.runner.Run(ctx, repository, "status", "--porcelain=v1", "-z")
	if err != nil {
		return nil, err
	}
	return parseChanges(out)
}

func parseChanges(output string) ([]Change, error) {
	records := strings.Split(output, "\x00")
	changes := make([]Change, 0, len(records))
	for index := 0; index < len(records); index++ {
		record := records[index]
		if record == "" {
			continue
		}
		if len(record) < 4 {
			return nil, errors.New("invalid git status record")
		}
		change := Change{Path: record[3:], Status: normalizeStatus(record[:2])}
		if change.Status == "renamed" {
			if index+1 >= len(records) || records[index+1] == "" {
				return nil, errors.New("invalid git rename record")
			}
			change.PreviousPath = records[index+1]
			index++
		}
		changes = append(changes, change)
	}
	return changes, nil
}

func normalizeStatus(value string) string {
	switch {
	case strings.ContainsAny(value, "RC"):
		return "renamed"
	case strings.Contains(value, "D"):
		return "deleted"
	case value == "??":
		return "untracked"
	case strings.Contains(value, "A"):
		return "added"
	default:
		return "modified"
	}
}

func (p Provider) FileContents(
	ctx context.Context,
	repository string,
	path string,
) (string, string, error) {
	clean, err := cleanDiffPath(path)
	if err != nil {
		return "", "", err
	}
	change, err := p.findChange(ctx, repository, filepath.ToSlash(clean))
	if err != nil {
		return "", "", err
	}
	original, err := p.originalContent(ctx, repository, change)
	if err != nil {
		return "", "", err
	}
	modified, err := workingContent(repository, clean, change.Status == "deleted")
	if err != nil {
		return "", "", err
	}
	if err := validateDiffContent(original, modified); err != nil {
		return "", "", err
	}
	return original, modified, nil
}

func (p Provider) findChange(
	ctx context.Context,
	repository string,
	path string,
) (Change, error) {
	changes, err := p.ChangedFiles(ctx, repository)
	if err != nil {
		return Change{}, err
	}
	for _, change := range changes {
		if change.Path == path {
			return change, nil
		}
	}
	return Change{}, ErrChangeNotFound
}

func (p Provider) originalContent(
	ctx context.Context,
	repository string,
	change Change,
) (string, error) {
	if change.Status == "added" || change.Status == "untracked" {
		return "", nil
	}
	path := change.Path
	if change.PreviousPath != "" {
		path = change.PreviousPath
	}
	return p.runner.Run(ctx, repository, "show", "HEAD:"+path)
}

func workingContent(repository, relative string, deleted bool) (string, error) {
	if deleted {
		return "", nil
	}
	root, err := filepath.EvalSymlinks(repository)
	if err != nil {
		return "", err
	}
	target := filepath.Join(root, relative)
	canonical, err := filepath.EvalSymlinks(target)
	if err != nil {
		return "", err
	}
	if !withinRoot(root, canonical) {
		return "", ErrInvalidDiffPath
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", ErrInvalidDiffPath
	}
	if info.Size() > maxDiffFileSize {
		return "", ErrDiffTooLarge
	}
	content, err := os.ReadFile(canonical)
	return string(content), err
}

func cleanDiffPath(path string) (string, error) {
	if path == "" || strings.ContainsRune(path, '\x00') || absoluteDiffPath(path) {
		return "", ErrInvalidDiffPath
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", ErrInvalidDiffPath
	}
	return clean, nil
}

func absoluteDiffPath(path string) bool {
	return filepath.IsAbs(path) || filepath.VolumeName(path) != "" ||
		strings.HasPrefix(path, "/") || strings.HasPrefix(path, `\`)
}

func withinRoot(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func validateDiffContent(values ...string) error {
	for _, value := range values {
		if len(value) > maxDiffFileSize {
			return ErrDiffTooLarge
		}
		if bytes.IndexByte([]byte(value), 0) >= 0 {
			return ErrBinaryDiff
		}
	}
	return nil
}
