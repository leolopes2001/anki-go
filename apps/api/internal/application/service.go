package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/local/anki-vocabulary/apps/api/internal/domain/model"
	"github.com/local/anki-vocabulary/apps/api/internal/domain/port"
)

type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

type Service struct {
	repository    port.Repository
	ai            port.AIProvider
	tts           port.TTSProvider
	pronunciation port.PronunciationProvider
	images        port.ImageProvider
	anki          port.AnkiProvider
	media         port.MediaStore
}

func NewService(repository port.Repository, ai port.AIProvider, tts port.TTSProvider, pronunciation port.PronunciationProvider, images port.ImageProvider, anki port.AnkiProvider, media port.MediaStore) *Service {
	return &Service{repository: repository, ai: ai, tts: tts, pronunciation: pronunciation, images: images, anki: anki, media: media}
}

func (s *Service) GenerateVocabulary(ctx context.Context, topic, level string) ([]string, error) {
	topic = strings.TrimSpace(topic)
	level = strings.ToUpper(strings.TrimSpace(level))
	if topic == "" || len(topic) > 120 {
		return nil, &Error{Code: "INVALID_CONTEXT", Message: "Context must contain between 1 and 120 characters."}
	}
	if level != "B1" && level != "B2" && level != "C1" {
		return nil, &Error{Code: "INVALID_LEVEL", Message: "Level must be B1, B2, or C1."}
	}
	seen, err := s.repository.SeenWords(ctx)
	if err != nil {
		return nil, err
	}
	words, err := s.ai.GenerateVocabulary(ctx, topic, level, 10, seen)
	if err != nil {
		return nil, &Error{Code: "VOCABULARY_GENERATION_FAILED", Message: "Unable to generate vocabulary."}
	}
	words = uniqueWords(words, seen, 10)
	if len(words) != 10 {
		return nil, &Error{Code: "VOCABULARY_GENERATION_FAILED", Message: "The AI provider did not return 10 new words."}
	}
	now := time.Now().UTC()
	records := make([]model.WordRecord, 0, len(words))
	for _, word := range words {
		records = append(records, model.WordRecord{Word: word, Context: topic, Level: level, Status: "PRESENTED", PresentedAt: now})
	}
	if err := s.repository.RecordPresented(ctx, records); err != nil {
		return nil, err
	}
	return words, nil
}

func (s *Service) CurrentPackage(ctx context.Context) (*model.Package, error) {
	return s.repository.CurrentPackage(ctx)
}

func (s *Service) AddWords(ctx context.Context, words []string) (*model.Package, error) {
	cleaned := uniqueWords(words, nil, 50)
	if len(cleaned) == 0 {
		return nil, &Error{Code: "INVALID_WORDS", Message: "Select at least one word."}
	}
	current, err := s.repository.CurrentPackage(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if current == nil {
		current = &model.Package{ID: newID("pkg"), Status: model.PackageOpen, Items: []model.PackageItem{}, CreatedAt: now}
	}
	if current.Status != model.PackageOpen {
		return nil, &Error{Code: "PACKAGE_NOT_OPEN", Message: "The current package can no longer be changed."}
	}
	existing := make(map[string]bool, len(current.Items))
	for _, item := range current.Items {
		existing[strings.ToLower(item.Word)] = true
	}
	for _, word := range cleaned {
		if !existing[strings.ToLower(word)] {
			current.Items = append(current.Items, model.PackageItem{Word: word})
		}
	}
	current.UpdatedAt = now
	if err := s.repository.SavePackage(ctx, current); err != nil {
		return nil, err
	}
	return current, nil
}

func (s *Service) RemoveWord(ctx context.Context, word string) (*model.Package, error) {
	current, err := s.repository.CurrentPackage(ctx)
	if err != nil || current == nil {
		return current, err
	}
	if current.Status != model.PackageOpen {
		return nil, &Error{Code: "PACKAGE_NOT_OPEN", Message: "The current package can no longer be changed."}
	}
	filtered := current.Items[:0]
	for _, item := range current.Items {
		if !strings.EqualFold(item.Word, word) {
			filtered = append(filtered, item)
		}
	}
	current.Items = filtered
	current.UpdatedAt = time.Now().UTC()
	return current, s.repository.SavePackage(ctx, current)
}

func (s *Service) GenerateSentences(ctx context.Context) (*model.Package, error) {
	current, err := s.repository.CurrentPackage(ctx)
	if err != nil {
		return nil, err
	}
	if current == nil || len(current.Items) == 0 {
		return nil, &Error{Code: "PACKAGE_NOT_FOUND", Message: "Add words before finalizing a package."}
	}
	if current.Status != model.PackageOpen {
		return nil, &Error{Code: "PACKAGE_NOT_OPEN", Message: "This package was already finalized."}
	}
	current.Status = model.PackageGeneratingSentences
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.SavePackage(ctx, current); err != nil {
		return nil, err
	}
	for index := range current.Items {
		sentence, generationErr := s.ai.GenerateSentence(ctx, current.Items[index].Word, "")
		if generationErr != nil {
			current.Status = model.PackageOpen
			_ = s.repository.SavePackage(ctx, current)
			return nil, &Error{Code: "SENTENCE_GENERATION_FAILED", Message: "Unable to generate all sentences."}
		}
		current.Items[index].Sentence = strings.TrimSpace(sentence)
	}
	current.Status = model.PackageReviewing
	current.UpdatedAt = time.Now().UTC()
	return current, s.repository.SavePackage(ctx, current)
}

func (s *Service) Review(ctx context.Context, packageID string) (*model.Package, error) {
	current, err := s.repository.FindPackage(ctx, packageID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, &Error{Code: "PACKAGE_NOT_FOUND", Message: "Package not found."}
	}
	return current, nil
}

func (s *Service) RegenerateSentence(ctx context.Context, packageID, word string) (*model.Package, error) {
	current, err := s.Review(ctx, packageID)
	if err != nil {
		return nil, err
	}
	if current.Status != model.PackageReviewing {
		return nil, &Error{Code: "PACKAGE_NOT_IN_REVIEW", Message: "Package is not in review."}
	}
	for index := range current.Items {
		if strings.EqualFold(current.Items[index].Word, word) {
			previous := strings.TrimSpace(current.Items[index].Sentence)
			forbidden := previous
			var sentence string
			for range 3 {
				candidate, generationErr := s.ai.GenerateSentence(ctx, current.Items[index].Word, forbidden)
				if generationErr != nil {
					return nil, &Error{Code: "SENTENCE_GENERATION_FAILED", Message: "Unable to generate a new sentence."}
				}
				sentence = strings.TrimSpace(candidate)
				if sentence != "" && !sentencesTooSimilar(sentence, previous) {
					break
				}
				if sentence != "" {
					forbidden += "\n" + sentence
				}
				sentence = ""
			}
			if sentence == "" {
				return nil, &Error{Code: "SENTENCE_GENERATION_FAILED", Message: "The AI repeated the same sentence. Try again."}
			}
			current.Items[index].Sentence = sentence
			current.UpdatedAt = time.Now().UTC()
			return current, s.repository.SavePackage(ctx, current)
		}
	}
	return nil, &Error{Code: "WORD_NOT_FOUND", Message: "Word not found in package."}
}

func sentencesTooSimilar(first, second string) bool {
	firstWords := sentenceWords(first)
	secondWords := sentenceWords(second)
	if len(firstWords) == 0 || len(secondWords) == 0 {
		return strings.EqualFold(strings.TrimSpace(first), strings.TrimSpace(second))
	}
	intersection := 0
	for word := range firstWords {
		if secondWords[word] {
			intersection++
		}
	}
	return float64(2*intersection)/float64(len(firstWords)+len(secondWords)) >= 0.7
}

func sentenceWords(sentence string) map[string]bool {
	words := strings.FieldsFunc(strings.ToLower(sentence), func(character rune) bool {
		return !unicode.IsLetter(character) && !unicode.IsNumber(character)
	})
	unique := make(map[string]bool, len(words))
	for _, word := range words {
		unique[word] = true
	}
	return unique
}

func (s *Service) ConfirmReview(ctx context.Context, packageID string) ([]model.QueueItem, error) {
	current, err := s.Review(ctx, packageID)
	if err != nil {
		return nil, err
	}
	if current.Status != model.PackageReviewing {
		return nil, &Error{Code: "PACKAGE_NOT_IN_REVIEW", Message: "Package is not in review."}
	}
	now := time.Now().UTC()
	items := make([]model.QueueItem, 0, len(current.Items))
	for _, item := range current.Items {
		if strings.TrimSpace(item.Sentence) == "" {
			return nil, &Error{Code: "REVIEW_INCOMPLETE", Message: "Every word must have an approved sentence."}
		}
		items = append(items, model.QueueItem{ID: newID("card"), PackageID: current.ID, Word: item.Word, Sentence: item.Sentence, Status: model.QueuePending, CreatedAt: now})
	}
	if err := s.repository.Enqueue(ctx, items); err != nil {
		return nil, err
	}
	current.Status = model.PackageCompleted
	current.UpdatedAt = now
	if err := s.repository.SavePackage(ctx, current); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Service) Queue(ctx context.Context) ([]model.QueueItem, error) {
	return s.repository.Queue(ctx)
}

func (s *Service) Retry(ctx context.Context, id string) error {
	items, err := s.repository.Queue(ctx)
	if err != nil {
		return err
	}
	for index := range items {
		if items[index].ID == id {
			if items[index].Status != model.QueueError {
				return &Error{Code: "QUEUE_ITEM_NOT_RETRYABLE", Message: "Only failed items can be retried."}
			}
			items[index].Status = model.QueuePending
			items[index].Error = ""
			items[index].Progress = "Waiting"
			return s.repository.SaveQueueItem(ctx, &items[index])
		}
	}
	return &Error{Code: "QUEUE_ITEM_NOT_FOUND", Message: "Queue item not found."}
}

func (s *Service) DeleteQueueItem(ctx context.Context, id string) error {
	items, err := s.repository.Queue(ctx)
	if err != nil {
		return err
	}
	for index := range items {
		if items[index].ID != id {
			continue
		}
		if items[index].Status == model.QueueProcessing || items[index].Status == model.QueueSendingToAnki {
			return &Error{Code: "QUEUE_ITEM_BUSY", Message: "Wait for this item to finish processing before deleting it."}
		}
		return s.repository.DeleteQueueItem(ctx, id)
	}
	return &Error{Code: "QUEUE_ITEM_NOT_FOUND", Message: "Queue item not found."}
}

func (s *Service) Media(directory, name string) ([]byte, string, error) {
	return s.media.Read(directory, name)
}

func (s *Service) ProcessNext(ctx context.Context) (bool, error) {
	item, err := s.repository.ClaimPending(ctx)
	if err != nil || item == nil {
		return false, err
	}
	if err := s.process(ctx, item); err != nil {
		item.Status = model.QueueError
		item.Progress = "Failed"
		item.Error = err.Error()
		return true, s.repository.SaveQueueItem(ctx, item)
	}
	now := time.Now().UTC()
	item.Status = model.QueueReady
	item.Progress = "Ready"
	item.ProcessedAt = &now
	item.Error = ""
	return true, s.repository.SaveQueueItem(ctx, item)
}

func (s *Service) process(ctx context.Context, item *model.QueueItem) error {
	var err error
	item.IPA, err = s.pronunciation.GetIPA(ctx, item.Word)
	if err != nil {
		return fmt.Errorf("pronunciation generation failed: %w", err)
	}
	item.Progress = "Generating definition"
	if err := s.repository.SaveQueueItem(ctx, item); err != nil {
		return err
	}
	item.Definition, err = s.ai.GenerateDefinition(ctx, item.Word)
	if err != nil {
		return fmt.Errorf("definition generation failed: %w", err)
	}
	item.Progress = "Generating audio"
	if err := s.repository.SaveQueueItem(ctx, item); err != nil {
		return err
	}
	audio, err := s.tts.Generate(ctx, item.Sentence)
	if err != nil {
		return fmt.Errorf("audio generation failed: %w", err)
	}
	item.SentenceAudio, err = s.media.SaveAudio(item.ID, audio)
	if err != nil {
		return err
	}
	item.Progress = "Generating image"
	if err := s.repository.SaveQueueItem(ctx, item); err != nil {
		return err
	}
	if err := s.generateImage(ctx, item, false); err != nil {
		return fmt.Errorf("image generation failed: %w", err)
	}
	return nil
}

func (s *Service) GenerateImage(ctx context.Context, id string) (*model.QueueItem, error) {
	items, err := s.repository.Queue(ctx)
	if err != nil {
		return nil, err
	}
	for index := range items {
		if items[index].ID != id {
			continue
		}
		if items[index].Status != model.QueueReady {
			return nil, &Error{Code: "QUEUE_ITEM_NOT_READY", Message: "Only ready cards can generate an image separately."}
		}
		if err := s.generateImage(ctx, &items[index], true); err != nil {
			return nil, &Error{Code: "IMAGE_GENERATION_FAILED", Message: "Unable to find a usable image. Try again."}
		}
		if err := s.repository.SaveQueueItem(ctx, &items[index]); err != nil {
			return nil, err
		}
		return &items[index], nil
	}
	return nil, &Error{Code: "QUEUE_ITEM_NOT_FOUND", Message: "Queue item not found."}
}

func (s *Service) generateImage(ctx context.Context, item *model.QueueItem, force bool) error {
	query := item.Word
	decision, err := s.ai.AnalyzeImage(ctx, item.Word)
	if err == nil {
		if !force && !decision.UseImage {
			item.Image = nil
			return nil
		}
		candidate := strings.TrimSpace(decision.Query)
		if candidate != "" && strings.Contains(strings.ToLower(candidate), strings.ToLower(item.Word)) {
			query = candidate
		}
	}
	candidates, err := s.images.Search(ctx, query, 5)
	if err != nil {
		return err
	}
	if force && item.ImageSourceID != 0 {
		alternatives := make([]model.ImageCandidate, 0, len(candidates)-1)
		for _, candidate := range candidates {
			if candidate.ID != item.ImageSourceID {
				alternatives = append(alternatives, candidate)
			}
		}
		candidates = alternatives
	}
	if len(candidates) == 0 {
		return errors.New("no alternative image candidates found")
	}
	selectedIndex, err := s.ai.SelectImage(ctx, item.Word, candidates)
	if err != nil {
		return err
	}
	if force && item.Image != nil && item.ImageSourceID == 0 && len(candidates) > 1 {
		alternatives := make([]model.ImageCandidate, 0, len(candidates)-1)
		for _, candidate := range candidates {
			if candidate.Index != selectedIndex {
				alternatives = append(alternatives, candidate)
			}
		}
		selectedIndex, err = s.ai.SelectImage(ctx, item.Word, alternatives)
		if err != nil {
			return err
		}
		candidates = alternatives
	}
	var selected *model.ImageCandidate
	for index := range candidates {
		if candidates[index].Index == selectedIndex {
			selected = &candidates[index]
			break
		}
	}
	if selected == nil {
		return errors.New("selected image candidate not found")
	}
	content, extension, err := s.images.Download(ctx, *selected)
	if err != nil {
		return err
	}
	path, err := s.media.SaveImage(item.ID, extension, content)
	if err != nil {
		return err
	}
	item.Image = &path
	item.ImageRevision = strconv.FormatInt(time.Now().UnixNano(), 10)
	item.ImageSourceID = selected.ID
	return nil
}

func (s *Service) History(ctx context.Context) ([]model.HistoryItem, error) {
	return s.repository.History(ctx)
}

func (s *Service) Decks(ctx context.Context) ([]string, error) {
	decks, err := s.anki.GetDecks(ctx)
	if err != nil {
		return nil, &Error{Code: "ANKI_UNAVAILABLE", Message: "AnkiConnect is unavailable."}
	}
	return decks, nil
}

func (s *Service) SendToAnki(ctx context.Context, deck string, ids []string) ([]model.QueueItem, error) {
	deck = strings.TrimSpace(deck)
	if deck == "" {
		return nil, &Error{Code: "INVALID_DECK", Message: "Select a destination deck."}
	}
	selected := make(map[string]bool, len(ids))
	for _, id := range ids {
		selected[id] = true
	}
	items, err := s.repository.Queue(ctx)
	if err != nil {
		return nil, err
	}
	sent := []model.QueueItem{}
	for index := range items {
		if items[index].Status != model.QueueReady || (len(selected) > 0 && !selected[items[index].ID]) {
			continue
		}
		items[index].Status = model.QueueSendingToAnki
		if err := s.repository.SaveQueueItem(ctx, &items[index]); err != nil {
			return sent, err
		}
		noteID, createErr := s.anki.CreateNote(ctx, items[index], deck)
		if createErr != nil {
			items[index].Status = model.QueueError
			items[index].Error = createErr.Error()
			_ = s.repository.SaveQueueItem(ctx, &items[index])
			continue
		}
		now := time.Now().UTC()
		items[index].Status = model.QueueSent
		items[index].Deck = deck
		items[index].AnkiNoteID = noteID
		items[index].SentAt = &now
		if err := s.repository.SaveQueueItem(ctx, &items[index]); err != nil {
			return sent, err
		}
		if err := s.repository.AppendHistory(ctx, model.HistoryItem{QueueItem: items[index]}); err != nil {
			return sent, err
		}
		sent = append(sent, items[index])
	}
	return sent, nil
}

func (s *Service) Health(ctx context.Context) model.Health {
	health := model.Health{API: "ok", DeepSeek: "not_configured", ElevenLabs: "not_configured", Anki: "disconnected"}
	if s.ai.AIHealth(ctx) == nil {
		health.DeepSeek = "configured"
	}
	if s.tts.Configured() {
		health.ElevenLabs = "configured"
	}
	if s.anki.AnkiHealth(ctx) == nil {
		health.Anki = "connected"
	}
	return health
}

func (s *Service) ImageStatus() string {
	if s.images.ImageConfigured() {
		return "configured"
	}
	return "not_configured"
}

func uniqueWords(words, excluded []string, limit int) []string {
	seen := make(map[string]bool, len(excluded)+len(words))
	for _, word := range excluded {
		seen[strings.ToLower(strings.TrimSpace(word))] = true
	}
	result := make([]string, 0, min(limit, len(words)))
	for _, word := range words {
		word = strings.ToLower(strings.TrimSpace(word))
		if word == "" || len(word) > 60 || seen[word] || strings.ContainsAny(word, "0123456789<>/\\") {
			continue
		}
		seen[word] = true
		result = append(result, word)
		if len(result) == limit {
			break
		}
	}
	return result
}

func newID(prefix string) string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		panic(errors.New("secure random source unavailable"))
	}
	return prefix + "_" + hex.EncodeToString(bytes)
}
