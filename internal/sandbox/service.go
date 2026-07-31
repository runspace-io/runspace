// Package sandbox exposes bounded, repository-relative read access to a
// run-scoped checkout. It deliberately has no write or command execution API.
package sandbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

var (
	ErrInvalidPath = errors.New("invalid resource path")
	ErrNotFound    = errors.New("resource file not found")
	ErrTooLarge    = errors.New("resource file is too large")
	ErrBinary      = errors.New("resource file is binary")
	ErrSymlink     = errors.New("resource symlinks are not readable")
)

const (
	defaultMaxReadBytes = 1 << 20
	defaultMaxEntries   = 10000
)

type Entry struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Size     int64  `json:"size"`
	Mode     uint32 `json:"mode"`
	Ignored  bool   `json:"ignored"`
	Readable bool   `json:"readable"`
	Reason   string `json:"reason,omitempty"`
}

type File struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Content string `json:"content"`
}

type RootResolver interface {
	Root(context.Context, string, string) (string, error)
}

// LayoutResolver maps a repository ID to <base>/<repositoryID>. IDs are
// treated as single path components, preventing user-controlled path escape.
type LayoutResolver struct{ base string }

func NewLayoutResolver(base string) (LayoutResolver, error) {
	if strings.TrimSpace(base) == "" {
		return LayoutResolver{}, ErrInvalidPath
	}
	abs, err := filepath.Abs(base)
	if err != nil {
		return LayoutResolver{}, fmt.Errorf("resolve resource root: %w", err)
	}
	return LayoutResolver{base: filepath.Clean(abs)}, nil
}

func (r LayoutResolver) Root(_ context.Context, _ string, repositoryID string) (string, error) {
	if !validComponent(repositoryID) {
		return "", ErrInvalidPath
	}
	return filepath.Join(r.base, repositoryID), nil
}

type Config struct {
	MaxReadBytes int64
	MaxEntries   int
}

type Service struct {
	resolver     RootResolver
	maxReadBytes int64
	maxEntries   int
}

func NewService(resolver RootResolver, config Config) (*Service, error) {
	if resolver == nil {
		return nil, errors.New("resource root resolver is required")
	}
	if config.MaxReadBytes <= 0 {
		config.MaxReadBytes = defaultMaxReadBytes
	}
	if config.MaxEntries <= 0 {
		config.MaxEntries = defaultMaxEntries
	}
	return &Service{resolver: resolver, maxReadBytes: config.MaxReadBytes, maxEntries: config.MaxEntries}, nil
}

func (s *Service) Tree(ctx context.Context, workspaceID, repositoryID, relative string) ([]Entry, error) {
	root, target, err := s.resolve(ctx, workspaceID, repositoryID, relative)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(target)
	if err != nil {
		return nil, mapPathError(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, ErrSymlink
	}
	if !info.IsDir() {
		return nil, ErrInvalidPath
	}
	items, err := os.ReadDir(target)
	if err != nil {
		return nil, err
	}
	if len(items) > s.maxEntries {
		return nil, ErrTooLarge
	}
	entries := make([]Entry, 0, len(items))
	for _, item := range items {
		entry, entryErr := s.entry(root, target, item)
		if entryErr != nil {
			return nil, entryErr
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Kind == entries[j].Kind {
			return entries[i].Path < entries[j].Path
		}
		return entries[i].Kind == "directory"
	})
	return entries, nil
}

func (s *Service) entry(root, parent string, item os.DirEntry) (Entry, error) {
	name := item.Name()
	rel, err := filepath.Rel(root, filepath.Join(parent, name))
	if err != nil {
		return Entry{}, err
	}
	path := filepath.ToSlash(rel)
	info, err := os.Lstat(filepath.Join(parent, name))
	if err != nil {
		return Entry{}, err
	}
	entry := Entry{Path: path, Name: name, Size: info.Size(), Mode: uint32(info.Mode().Perm()), Readable: true}
	if info.Mode()&os.ModeSymlink != 0 {
		entry.Kind, entry.Readable, entry.Reason = "symlink", false, "symlink is not traversed"
		return entry, nil
	}
	switch {
	case info.IsDir():
		entry.Kind = "directory"
	case info.Mode().IsRegular():
		entry.Kind = "file"
		if info.Size() > s.maxReadBytes {
			entry.Readable, entry.Reason = false, "file exceeds read limit"
		}
	default:
		entry.Kind, entry.Readable, entry.Reason = "special", false, "special file is not readable"
	}
	if name == ".git" {
		entry.Ignored, entry.Readable, entry.Reason = true, false, "Git metadata is hidden"
	}
	if name == ".stfolder" || name == ".stignore" {
		entry.Ignored, entry.Readable, entry.Reason = true, false, "sync metadata is hidden"
	}
	return entry, nil
}

func (s *Service) Read(ctx context.Context, workspaceID, repositoryID, relative string) (File, error) {
	_, target, err := s.resolve(ctx, workspaceID, repositoryID, relative)
	if err != nil {
		return File{}, err
	}
	info, err := os.Lstat(target)
	if err != nil {
		return File{}, mapPathError(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return File{}, ErrSymlink
	}
	if !info.Mode().IsRegular() {
		return File{}, ErrInvalidPath
	}
	if info.Size() > s.maxReadBytes {
		return File{}, ErrTooLarge
	}
	data, err := readBounded(target, s.maxReadBytes)
	if err != nil {
		return File{}, err
	}
	cleanRelative, err := relativePath(relative)
	if err != nil {
		return File{}, err
	}
	return File{Path: filepath.ToSlash(cleanRelative), Size: int64(len(data)), Content: string(data)}, nil
}

func (s *Service) resolve(ctx context.Context, workspaceID, repositoryID, relative string) (string, string, error) {
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	root, err := s.resolver.Root(ctx, workspaceID, repositoryID)
	if err != nil {
		return "", "", err
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", "", mapPathError(err)
	}
	cleanRelative, err := relativePath(relative)
	if err != nil {
		return "", "", err
	}
	target := filepath.Join(canonicalRoot, filepath.FromSlash(cleanRelative))
	canonicalTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		return "", "", mapPathError(err)
	}
	if !within(canonicalRoot, canonicalTarget) {
		return "", "", ErrInvalidPath
	}
	return canonicalRoot, canonicalTarget, nil
}

func relativePath(value string) (string, error) {
	value = strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	if strings.ContainsRune(value, '\x00') {
		return "", ErrInvalidPath
	}
	if value == "" || value == "." {
		return ".", nil
	}
	if strings.HasPrefix(value, "/") || filepath.IsAbs(value) {
		return "", ErrInvalidPath
	}
	parts := strings.Split(value, "/")
	for _, part := range parts {
		if part == ".." || part == "." || part == "" {
			return "", ErrInvalidPath
		}
	}
	return strings.Join(parts, "/"), nil
}

func readBounded(path string, max int64) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, ErrTooLarge
	}
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return nil, ErrBinary
	}
	return data, nil
}

func validComponent(value string) bool {
	return value != "" && value != "." && value != ".." && !strings.ContainsAny(value, `/\\`)
}

func within(root, target string) bool {
	if root == target {
		return true
	}
	return strings.HasPrefix(target, root+string(filepath.Separator))
}

func mapPathError(err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	}
	return err
}
