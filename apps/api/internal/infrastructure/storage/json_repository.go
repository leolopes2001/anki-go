package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/local/anki-vocabulary/apps/api/internal/domain/model"
)

type JSONRepository struct {
	dataDir string
	mu      sync.Mutex
}

func NewJSONRepository(dataDir string) (*JSONRepository, error) {
	if err := os.MkdirAll(filepath.Join(dataDir, "cards"), 0o755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	repository := &JSONRepository{dataDir: dataDir}
	for _, file := range []string{"words.json", "packages.json", "queue.json", "history.json"} {
		path := filepath.Join(dataDir, file)
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			if err := writeAtomic(path, []any{}); err != nil {
				return nil, err
			}
		}
	}
	return repository, nil
}

func (r *JSONRepository) SeenWords(_ context.Context) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var records []model.WordRecord
	if err := readJSON(filepath.Join(r.dataDir, "words.json"), &records); err != nil {
		return nil, err
	}
	words := make([]string, 0, len(records))
	for _, record := range records {
		words = append(words, record.Word)
	}
	return words, nil
}

func (r *JSONRepository) RecordPresented(_ context.Context, incoming []model.WordRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	path := filepath.Join(r.dataDir, "words.json")
	var records []model.WordRecord
	if err := readJSON(path, &records); err != nil {
		return err
	}
	existing := make(map[string]bool, len(records))
	for _, record := range records {
		existing[strings.ToLower(record.Word)] = true
	}
	for _, record := range incoming {
		if !existing[strings.ToLower(record.Word)] {
			records = append(records, record)
		}
	}
	return writeAtomic(path, records)
}

func (r *JSONRepository) CurrentPackage(_ context.Context) (*model.Package, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	packages, err := r.readPackages()
	if err != nil {
		return nil, err
	}
	for index := len(packages) - 1; index >= 0; index-- {
		if packages[index].Status != model.PackageCompleted {
			copy := packages[index]
			return &copy, nil
		}
	}
	return nil, nil
}

func (r *JSONRepository) FindPackage(_ context.Context, id string) (*model.Package, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	packages, err := r.readPackages()
	if err != nil {
		return nil, err
	}
	for _, item := range packages {
		if item.ID == id {
			copy := item
			return &copy, nil
		}
	}
	return nil, nil
}

func (r *JSONRepository) SavePackage(_ context.Context, current *model.Package) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	packages, err := r.readPackages()
	if err != nil {
		return err
	}
	found := false
	for index := range packages {
		if packages[index].ID == current.ID {
			packages[index] = *current
			found = true
			break
		}
	}
	if !found {
		packages = append(packages, *current)
	}
	return writeAtomic(filepath.Join(r.dataDir, "packages.json"), packages)
}

func (r *JSONRepository) Enqueue(_ context.Context, incoming []model.QueueItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	path := filepath.Join(r.dataDir, "queue.json")
	var items []model.QueueItem
	if err := readJSON(path, &items); err != nil {
		return err
	}
	items = append(items, incoming...)
	return writeAtomic(path, items)
}

func (r *JSONRepository) Queue(_ context.Context) ([]model.QueueItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var items []model.QueueItem
	err := readJSON(filepath.Join(r.dataDir, "queue.json"), &items)
	return items, err
}

func (r *JSONRepository) ClaimPending(_ context.Context) (*model.QueueItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	path := filepath.Join(r.dataDir, "queue.json")
	var items []model.QueueItem
	if err := readJSON(path, &items); err != nil {
		return nil, err
	}
	for index := range items {
		if items[index].Status == model.QueuePending {
			items[index].Status = model.QueueProcessing
			items[index].Progress = "Generating pronunciation"
			items[index].Attempts++
			if err := writeAtomic(path, items); err != nil {
				return nil, err
			}
			copy := items[index]
			return &copy, nil
		}
	}
	return nil, nil
}

func (r *JSONRepository) SaveQueueItem(_ context.Context, current *model.QueueItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	path := filepath.Join(r.dataDir, "queue.json")
	var items []model.QueueItem
	if err := readJSON(path, &items); err != nil {
		return err
	}
	for index := range items {
		if items[index].ID == current.ID {
			items[index] = *current
			return writeAtomic(path, items)
		}
	}
	return fmt.Errorf("queue item not found")
}

func (r *JSONRepository) DeleteQueueItem(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	path := filepath.Join(r.dataDir, "queue.json")
	var items []model.QueueItem
	if err := readJSON(path, &items); err != nil {
		return err
	}
	for index := range items {
		if items[index].ID == id {
			items = append(items[:index], items[index+1:]...)
			return writeAtomic(path, items)
		}
	}
	return fmt.Errorf("queue item not found")
}

func (r *JSONRepository) AppendHistory(_ context.Context, incoming model.HistoryItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	path := filepath.Join(r.dataDir, "history.json")
	var items []model.HistoryItem
	if err := readJSON(path, &items); err != nil {
		return err
	}
	items = append(items, incoming)
	return writeAtomic(path, items)
}

func (r *JSONRepository) History(_ context.Context) ([]model.HistoryItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var items []model.HistoryItem
	err := readJSON(filepath.Join(r.dataDir, "history.json"), &items)
	return items, err
}

func (r *JSONRepository) readPackages() ([]model.Package, error) {
	var packages []model.Package
	err := readJSON(filepath.Join(r.dataDir, "packages.json"), &packages)
	return packages, err
}

func readJSON(path string, target any) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(content, target); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func writeAtomic(path string, value any) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		temporary.Close()
		return fmt.Errorf("encode temporary file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	dir, err := os.Open(directory)
	if err == nil {
		defer dir.Close()
		if err := dir.Sync(); err != nil {
			return fmt.Errorf("sync directory: %w", err)
		}
	}
	return nil
}
