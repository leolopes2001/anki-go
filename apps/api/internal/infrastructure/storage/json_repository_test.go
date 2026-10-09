package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/local/anki-vocabulary/apps/api/internal/domain/model"
)

func TestJSONRepositoryPersistsPackageAndQueue(t *testing.T) {
	repository, err := NewJSONRepository(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	current := &model.Package{ID: "pkg_test", Status: model.PackageOpen, Items: []model.PackageItem{{Word: "reliable"}}, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := repository.SavePackage(ctx, current); err != nil {
		t.Fatal(err)
	}
	reloaded, err := repository.CurrentPackage(ctx)
	if err != nil || reloaded == nil || reloaded.Items[0].Word != "reliable" {
		t.Fatalf("unexpected persisted package: %#v, %v", reloaded, err)
	}
	item := model.QueueItem{ID: "card_test", Word: "reliable", Status: model.QueuePending}
	if err := repository.Enqueue(ctx, []model.QueueItem{item}); err != nil {
		t.Fatal(err)
	}
	claimed, err := repository.ClaimPending(ctx)
	if err != nil || claimed == nil || claimed.Status != model.QueueProcessing {
		t.Fatalf("unexpected claimed item: %#v, %v", claimed, err)
	}
}

func TestMediaStoreRejectsUnsafeID(t *testing.T) {
	store, err := NewMediaStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveAudio("../escape", []byte("audio")); err == nil {
		t.Fatal("expected unsafe media id to fail")
	}
	path, err := store.SaveAudio("card_safe", []byte("audio"))
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.ToSlash("storage/audio/card_safe.mp3") {
		t.Fatalf("unexpected relative path: %s", path)
	}
	if _, err := os.Stat(filepath.Join(store.root, "audio", "card_safe.mp3")); err != nil {
		t.Fatal(err)
	}
	content, contentType, err := store.Read("audio", "card_safe.mp3")
	if err != nil || string(content) != "audio" || contentType != "audio/mpeg" {
		t.Fatalf("unexpected media read: %q, %q, %v", content, contentType, err)
	}
	if _, _, err := store.Read("audio", "../card_safe.mp3"); err == nil {
		t.Fatal("expected unsafe media path to fail")
	}
}

func TestJSONRepositoryDeletesQueueItem(t *testing.T) {
	repository, err := NewJSONRepository(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	items := []model.QueueItem{
		{ID: "card_keep", Word: "keep", Status: model.QueueReady},
		{ID: "card_delete", Word: "delete", Status: model.QueueReady},
	}
	if err := repository.Enqueue(ctx, items); err != nil {
		t.Fatal(err)
	}
	if err := repository.DeleteQueueItem(ctx, "card_delete"); err != nil {
		t.Fatal(err)
	}
	remaining, err := repository.Queue(ctx)
	if err != nil || len(remaining) != 1 || remaining[0].ID != "card_keep" {
		t.Fatalf("unexpected queue after delete: %#v, %v", remaining, err)
	}
	if err := repository.DeleteQueueItem(ctx, "missing"); err == nil {
		t.Fatal("expected missing queue item to fail")
	}
}
