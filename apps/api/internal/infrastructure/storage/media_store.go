package storage

import (
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var safeID = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
var safeMediaName = regexp.MustCompile(`^[a-zA-Z0-9_-]+\.(mp3|jpg|jpeg|png|webp)$`)

type MediaStore struct {
	root string
}

func NewMediaStore(root string) (*MediaStore, error) {
	for _, directory := range []string{"audio", "images"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			return nil, fmt.Errorf("create media directory: %w", err)
		}
	}
	return &MediaStore{root: root}, nil
}

func (s *MediaStore) SaveAudio(id string, data []byte) (string, error) {
	return s.save("audio", id, ".mp3", data)
}

func (s *MediaStore) SaveImage(id, extension string, data []byte) (string, error) {
	extension = strings.ToLower(extension)
	if extension != ".jpg" && extension != ".jpeg" && extension != ".png" && extension != ".webp" {
		extension = ".jpg"
	}
	return s.save("images", id, extension, data)
}

func (s *MediaStore) Read(directory, name string) ([]byte, string, error) {
	if (directory != "audio" && directory != "images") || !safeMediaName.MatchString(name) {
		return nil, "", fmt.Errorf("invalid media path")
	}
	content, err := os.ReadFile(filepath.Join(s.root, directory, name))
	if err != nil {
		return nil, "", fmt.Errorf("read media: %w", err)
	}
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(name)))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return content, contentType, nil
}

func (s *MediaStore) save(directory, id, extension string, data []byte) (string, error) {
	if !safeID.MatchString(id) {
		return "", fmt.Errorf("invalid media id")
	}
	relative := filepath.Join(directory, id+extension)
	path := filepath.Join(s.root, relative)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("save media: %w", err)
	}
	return filepath.ToSlash(filepath.Join("storage", relative)), nil
}
