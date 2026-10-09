package application_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/local/anki-vocabulary/apps/api/internal/application"
	"github.com/local/anki-vocabulary/apps/api/internal/domain/model"
	"github.com/local/anki-vocabulary/apps/api/internal/infrastructure/storage"
)

type fakeProviders struct {
	sentenceCalls       map[string]int
	useImage            bool
	downloadedCandidate int
	repeatRegeneration  bool
}

func (f *fakeProviders) AIHealth(context.Context) error   { return nil }
func (f *fakeProviders) AnkiHealth(context.Context) error { return nil }
func (f *fakeProviders) Configured() bool                 { return true }
func (f *fakeProviders) GenerateVocabulary(_ context.Context, _ string, _ string, amount int, _ []string) ([]string, error) {
	words := []string{"awkward", "outcome", "barely", "reliable", "struggle", "afford", "throughout", "hesitate", "eventually", "aware"}
	return words[:amount], nil
}
func (f *fakeProviders) GenerateSentence(_ context.Context, word, avoid string) (string, error) {
	f.sentenceCalls[word]++
	if avoid != "" && f.repeatRegeneration {
		f.repeatRegeneration = false
		return strings.TrimSuffix(avoid, ".") + "!", nil
	}
	if avoid != "" {
		return fmt.Sprintf("A completely different situation uses %s in version %d.", word, f.sentenceCalls[word]), nil
	}
	return fmt.Sprintf("A natural sentence for %s version %d.", word, f.sentenceCalls[word]), nil
}
func (f *fakeProviders) GenerateDefinition(_ context.Context, word string) (string, error) {
	return "A definition for " + word, nil
}
func (f *fakeProviders) AnalyzeImage(context.Context, string) (model.ImageDecision, error) {
	return model.ImageDecision{UseImage: f.useImage}, nil
}
func (f *fakeProviders) SelectImage(_ context.Context, _ string, candidates []model.ImageCandidate) (int, error) {
	return candidates[1].Index, nil
}
func (f *fakeProviders) Generate(context.Context, string) ([]byte, error) {
	return []byte("audio"), nil
}
func (f *fakeProviders) GetIPA(_ context.Context, word string) (string, error) {
	return "/" + word + "/", nil
}
func (f *fakeProviders) ImageConfigured() bool { return true }
func (f *fakeProviders) Search(context.Context, string, int) ([]model.ImageCandidate, error) {
	return []model.ImageCandidate{{Index: 0, ID: 101}, {Index: 1, ID: 202}, {Index: 2, ID: 303}}, nil
}
func (f *fakeProviders) Download(_ context.Context, candidate model.ImageCandidate) ([]byte, string, error) {
	f.downloadedCandidate = candidate.ID
	return []byte("image"), ".jpg", nil
}
func (f *fakeProviders) GetDecks(context.Context) ([]string, error) {
	return []string{"English::Vocabulary"}, nil
}
func (f *fakeProviders) CreateNote(_ context.Context, item model.QueueItem, _ string) (string, error) {
	return "note_" + item.ID, nil
}

func TestVocabularyToAnkiLifecycle(t *testing.T) {
	ctx := context.Background()
	repository, err := storage.NewJSONRepository(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	media, err := storage.NewMediaStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	providers := &fakeProviders{sentenceCalls: map[string]int{}, useImage: true, repeatRegeneration: true}
	service := application.NewService(repository, providers, providers, providers, providers, providers, media)

	words, err := service.GenerateVocabulary(ctx, "Daily conversation", "B2")
	if err != nil || len(words) != 10 {
		t.Fatalf("generate vocabulary: %v, %v", words, err)
	}
	current, err := service.AddWords(ctx, words[:2])
	if err != nil || len(current.Items) != 2 {
		t.Fatalf("add words: %#v, %v", current, err)
	}
	current, err = service.GenerateSentences(ctx)
	if err != nil || current.Status != model.PackageReviewing {
		t.Fatalf("generate sentences: %#v, %v", current, err)
	}
	firstSentence := current.Items[0].Sentence
	current, err = service.RegenerateSentence(ctx, current.ID, words[0])
	if err != nil || current.Items[0].Sentence == firstSentence || current.Items[1].Sentence == "" {
		t.Fatalf("regenerate one sentence: %#v, %v", current, err)
	}
	if providers.sentenceCalls[words[0]] != 3 {
		t.Fatalf("expected near-duplicate sentence to be retried, got %d calls", providers.sentenceCalls[words[0]])
	}
	queue, err := service.ConfirmReview(ctx, current.ID)
	if err != nil || len(queue) != 2 {
		t.Fatalf("confirm review: %#v, %v", queue, err)
	}
	for range queue {
		processed, processErr := service.ProcessNext(ctx)
		if processErr != nil || !processed {
			t.Fatalf("process queue: %v, %v", processed, processErr)
		}
	}
	processed, err := service.Queue(ctx)
	if err != nil || len(processed) != 2 || processed[0].Image == nil || processed[1].Image == nil {
		t.Fatalf("processed cards should include images: %#v, %v", processed, err)
	}
	if providers.downloadedCandidate != 202 {
		t.Fatalf("expected only selected candidate 202 to be downloaded, got %d", providers.downloadedCandidate)
	}
	if processed[0].ImageSourceID != 202 || processed[0].ImageRevision == "" {
		t.Fatalf("expected persisted image identity, got %#v", processed[0])
	}
	previousRevision := processed[0].ImageRevision
	refreshed, err := service.GenerateImage(ctx, processed[0].ID)
	if err != nil {
		t.Fatalf("refresh image: %v", err)
	}
	if providers.downloadedCandidate != 303 || refreshed.ImageSourceID != 303 {
		t.Fatalf("expected refresh to download alternative candidate 303, got %d", providers.downloadedCandidate)
	}
	if refreshed.ImageRevision == previousRevision {
		t.Fatal("expected refresh to change image revision")
	}
	sent, err := service.SendToAnki(ctx, "English::Vocabulary", nil)
	if err != nil || len(sent) != 2 {
		t.Fatalf("send to Anki: %#v, %v", sent, err)
	}
	history, err := service.History(ctx)
	if err != nil || len(history) != 2 || history[0].Status != model.QueueSent {
		t.Fatalf("history: %#v, %v", history, err)
	}
	if err := service.DeleteQueueItem(ctx, queue[0].ID); err != nil {
		t.Fatalf("delete queue item: %v", err)
	}
	remaining, err := service.Queue(ctx)
	if err != nil || len(remaining) != 1 {
		t.Fatalf("queue after delete: %#v, %v", remaining, err)
	}
	history, err = service.History(ctx)
	if err != nil || len(history) != 2 {
		t.Fatalf("history after queue delete: %#v, %v", history, err)
	}
}
