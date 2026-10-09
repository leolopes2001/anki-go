package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/local/anki-vocabulary/apps/api/internal/domain/model"
)

const maxResponseSize = 10 << 20
const maxErrorResponseSize = 4 << 10

type Providers struct {
	client            *http.Client
	logger            *slog.Logger
	deepSeekURL       string
	deepSeekAPIKey    string
	deepSeekModel     string
	elevenLabsAPIKey  string
	elevenLabsVoiceID string
	ankiURL           string
	storageDir        string
	pixabayAPIKey     string
	ankiModelMu       sync.Mutex
	ankiModelReady    bool
}

func New(logger *slog.Logger, deepSeekURL, deepSeekAPIKey, deepSeekModel, elevenLabsAPIKey, elevenLabsVoiceID, ankiURL, storageDir, pixabayAPIKey string) *Providers {
	if logger == nil {
		logger = slog.Default()
	}
	return &Providers{
		client: &http.Client{Timeout: 30 * time.Second}, logger: logger, deepSeekURL: strings.TrimRight(deepSeekURL, "/"),
		deepSeekAPIKey: deepSeekAPIKey, deepSeekModel: deepSeekModel,
		elevenLabsAPIKey: elevenLabsAPIKey, elevenLabsVoiceID: elevenLabsVoiceID, ankiURL: ankiURL, storageDir: storageDir,
		pixabayAPIKey: pixabayAPIKey,
	}
}

func (p *Providers) AIHealth(ctx context.Context) error {
	if p.deepSeekAPIKey == "" {
		return errors.New("DEEPSEEK_API_KEY is not configured")
	}
	if p.deepSeekURL == "" {
		return errors.New("DEEPSEEK_URL is not configured")
	}
	if p.deepSeekModel == "" {
		return errors.New("DEEPSEEK_MODEL is not configured")
	}
	_ = ctx
	return nil
}

func (p *Providers) AnkiHealth(ctx context.Context) error {
	var version int
	return p.ankiCall(ctx, "version", map[string]any{}, &version)
}

func (p *Providers) Configured() bool {
	return p.elevenLabsAPIKey != "" && p.elevenLabsVoiceID != ""
}

func (p *Providers) GenerateVocabulary(ctx context.Context, topic, level string, amount int, seen []string) ([]string, error) {
	prompt := fmt.Sprintf("Generate exactly %d useful English vocabulary words for a %s learner in this context: %s. Exclude these previously seen words: %s. Return only JSON as {\"words\":[\"word\"]}. Use lowercase words or short phrasal verbs, no definitions.", amount, level, topic, strings.Join(seen, ", "))
	var result struct {
		Words []string `json:"words"`
	}
	if err := p.generateJSON(ctx, prompt, &result); err != nil {
		return nil, err
	}
	return result.Words, nil
}

func (p *Providers) GenerateSentence(ctx context.Context, word, avoid string) (string, error) {
	prompt := fmt.Sprintf("Write one natural English B1-C1 sentence that clearly demonstrates the meaning of %q. Keep it under 22 words.", word)
	if strings.TrimSpace(avoid) != "" {
		prompt += fmt.Sprintf(" The new sentence MUST be completely different from every sentence in these forbidden examples: %q. Use a different situation, subject, action, vocabulary, and grammatical structure. Do not paraphrase, reuse a metaphor, or slightly modify any forbidden sentence.", avoid)
	}
	prompt += " Return only JSON as {\"sentence\":\"...\"}."
	var result struct {
		Sentence string `json:"sentence"`
	}
	if err := p.generateJSON(ctx, prompt, &result); err != nil {
		return "", err
	}
	return result.Sentence, nil
}

func (p *Providers) GenerateDefinition(ctx context.Context, word string) (string, error) {
	prompt := fmt.Sprintf("Explain the English word or expression %q for a B1-C1 learner. Write a substantial learner-friendly definition in 2-3 sentences: state the core meaning, explain an important usage nuance, and mention another common sense only if useful. Do not translate and do not define the word using itself. Include up to 5 close English synonyms for the main meaning; use an empty array when there are no accurate synonyms. Return only JSON as {\"definition\":\"...\",\"synonyms\":[\"...\"]}.", word)
	var result struct {
		Definition string   `json:"definition"`
		Synonyms   []string `json:"synonyms"`
	}
	if err := p.generateJSON(ctx, prompt, &result); err != nil {
		return "", err
	}
	definition := strings.TrimSpace(result.Definition)
	synonyms := make([]string, 0, min(len(result.Synonyms), 5))
	seen := map[string]bool{strings.ToLower(strings.TrimSpace(word)): true}
	for _, synonym := range result.Synonyms {
		synonym = strings.TrimSpace(synonym)
		key := strings.ToLower(synonym)
		if synonym == "" || seen[key] {
			continue
		}
		seen[key] = true
		synonyms = append(synonyms, synonym)
		if len(synonyms) == 5 {
			break
		}
	}
	if len(synonyms) > 0 {
		if definition != "" && !strings.ContainsAny(definition[len(definition)-1:], ".!?") {
			definition += "."
		}
		definition += " Synonyms: " + strings.Join(synonyms, ", ") + "."
	}
	return definition, nil
}

func (p *Providers) AnalyzeImage(ctx context.Context, word string) (model.ImageDecision, error) {
	prompt := fmt.Sprintf("Decide whether an image materially helps learn the English word or expression %q. Consider only this word or expression, without sentence context. Abstract meanings that cannot be depicted clearly should not use an image. If useImage is true, query MUST begin with the exact word or expression and may add at most three words to clarify a common visual meaning. Return only JSON as {\"useImage\":true,\"query\":\"target word visual meaning\"}.", word)
	var result model.ImageDecision
	err := p.generateJSON(ctx, prompt, &result)
	return result, err
}

func (p *Providers) SelectImage(ctx context.Context, word string, candidates []model.ImageCandidate) (int, error) {
	if len(candidates) == 0 {
		return 0, errors.New("no image candidates")
	}
	metadata, err := json.Marshal(candidates)
	if err != nil {
		return 0, err
	}
	prompt := fmt.Sprintf("Choose the single Pixabay image that best teaches the English word or expression %q. Consider only this word or expression, without sentence context. Candidates: %s. Use tags for semantic relevance, reject misleading meanings, and prefer clear useful dimensions and stronger engagement only as tie-breakers. Return only JSON as {\"index\":0}, where index must match one candidate index.", word, metadata)
	var result struct {
		Index *int `json:"index"`
	}
	if err := p.generateJSON(ctx, prompt, &result); err != nil {
		return 0, err
	}
	if result.Index == nil {
		return 0, errors.New("DeepSeek image selection omitted index")
	}
	for _, candidate := range candidates {
		if candidate.Index == *result.Index {
			return *result.Index, nil
		}
	}
	return 0, fmt.Errorf("DeepSeek selected invalid image index %d", *result.Index)
}

func (p *Providers) generateJSON(ctx context.Context, prompt string, target any) error {
	if p.deepSeekAPIKey == "" {
		return errors.New("DEEPSEEK_API_KEY is not configured")
	}
	if p.deepSeekModel == "" {
		return errors.New("DEEPSEEK_MODEL is not configured")
	}
	body, _ := json.Marshal(map[string]any{
		"model": p.deepSeekModel,
		"messages": []map[string]string{
			{"role": "system", "content": "Return only valid JSON matching the requested schema."},
			{"role": "user", "content": prompt},
		},
		"temperature": 0.7,
		"top_p":       0.9,
		"stream":      false,
		"max_tokens":  4096,
	})
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	headers := map[string]string{"Authorization": "Bearer " + p.deepSeekAPIKey}
	if err := p.doJSONWithRetry(ctx, http.MethodPost, p.deepSeekURL+"/chat/completions", headers, body, &result); err != nil {
		return err
	}
	if len(result.Choices) == 0 || result.Choices[0].Message.Content == "" {
		return errors.New("DeepSeek returned no content")
	}
	if err := json.Unmarshal([]byte(result.Choices[0].Message.Content), target); err != nil {
		return fmt.Errorf("decode DeepSeek response: %w", err)
	}
	return nil
}

func (p *Providers) Generate(ctx context.Context, text string) ([]byte, error) {
	started := time.Now()
	if !p.Configured() {
		p.logger.Warn("ElevenLabs audio generation skipped", "operation", "generate_audio", "provider", "elevenlabs", "reason", "not_configured")
		return nil, errors.New("ElevenLabs is not configured")
	}
	p.logger.Info("ElevenLabs audio generation started", "operation", "generate_audio", "provider", "elevenlabs", "voiceId", p.elevenLabsVoiceID, "textLength", len(text), "model", "eleven_multilingual_v2")
	body, err := json.Marshal(map[string]any{"text": text, "model_id": "eleven_multilingual_v2", "output_format": "mp3_44100_128"})
	if err != nil {
		p.logger.Error("ElevenLabs request encoding failed", "operation", "generate_audio", "provider", "elevenlabs", "duration", time.Since(started), "error", err)
		return nil, err
	}
	endpoint := "https://api.elevenlabs.io/v1/text-to-speech/" + url.PathEscape(p.elevenLabsVoiceID)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		p.logger.Error("ElevenLabs request creation failed", "operation", "generate_audio", "provider", "elevenlabs", "duration", time.Since(started), "error", err)
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("xi-api-key", p.elevenLabsAPIKey)
	response, err := p.client.Do(request)
	if err != nil {
		p.logger.Error("ElevenLabs request failed", "operation", "generate_audio", "provider", "elevenlabs", "duration", time.Since(started), "error", err)
		return nil, err
	}
	defer response.Body.Close()
	requestID := response.Header.Get("request-id")
	p.logger.Info("ElevenLabs response received", "operation", "generate_audio", "provider", "elevenlabs", "status", response.StatusCode, "duration", time.Since(started), "requestId", requestID, "contentType", response.Header.Get("Content-Type"), "contentLength", response.ContentLength)
	if response.StatusCode != http.StatusOK {
		detail, readErr := io.ReadAll(io.LimitReader(response.Body, maxErrorResponseSize))
		p.logger.Error("ElevenLabs audio generation rejected", "operation", "generate_audio", "provider", "elevenlabs", "status", response.StatusCode, "duration", time.Since(started), "requestId", requestID, "response", strings.TrimSpace(string(detail)), "readError", readErr)
		return nil, fmt.Errorf("ElevenLabs returned %s", response.Status)
	}
	audio, err := io.ReadAll(io.LimitReader(response.Body, maxResponseSize+1))
	if err != nil {
		p.logger.Error("ElevenLabs audio response read failed", "operation", "generate_audio", "provider", "elevenlabs", "duration", time.Since(started), "requestId", requestID, "error", err)
		return nil, err
	}
	if len(audio) > maxResponseSize {
		err = errors.New("ElevenLabs audio exceeds size limit")
		p.logger.Error("ElevenLabs audio response rejected", "operation", "generate_audio", "provider", "elevenlabs", "duration", time.Since(started), "requestId", requestID, "bytes", len(audio), "error", err)
		return nil, err
	}
	p.logger.Info("ElevenLabs audio generation completed", "operation", "generate_audio", "provider", "elevenlabs", "duration", time.Since(started), "requestId", requestID, "bytes", len(audio))
	return audio, nil
}

func (p *Providers) GetIPA(ctx context.Context, word string) (string, error) {
	prompt := fmt.Sprintf("Give the standard English IPA pronunciation for %q. Include the surrounding slashes. Return only JSON as {\"ipa\":\"/.../\"}.", word)
	var result struct {
		IPA string `json:"ipa"`
	}
	if err := p.generateJSON(ctx, prompt, &result); err != nil {
		return "", err
	}
	if strings.TrimSpace(result.IPA) == "" {
		return "", errors.New("DeepSeek returned no pronunciation")
	}
	return strings.TrimSpace(result.IPA), nil
}

func (p *Providers) ImageConfigured() bool {
	return p.pixabayAPIKey != ""
}

func (p *Providers) Search(ctx context.Context, query string, limit int) ([]model.ImageCandidate, error) {
	if !p.ImageConfigured() {
		return nil, errors.New("PIXABAY_API_KEY is not configured")
	}
	limit = min(max(limit, 3), 20)
	endpoint := "https://pixabay.com/api/?key=" + url.QueryEscape(p.pixabayAPIKey) + "&q=" + url.QueryEscape(query) + "&image_type=photo&safesearch=true&lang=en&per_page=" + strconv.Itoa(limit)
	var result struct {
		Hits []struct {
			ID           int    `json:"id"`
			Tags         string `json:"tags"`
			User         string `json:"user"`
			WebformatURL string `json:"webformatURL"`
			ImageWidth   int    `json:"imageWidth"`
			ImageHeight  int    `json:"imageHeight"`
			Likes        int    `json:"likes"`
			Downloads    int    `json:"downloads"`
		} `json:"hits"`
	}
	if err := p.doJSONWithRetry(ctx, http.MethodGet, endpoint, nil, nil, &result); err != nil {
		return nil, err
	}
	if len(result.Hits) == 0 {
		return nil, errors.New("Pixabay returned no images")
	}
	candidates := make([]model.ImageCandidate, 0, len(result.Hits))
	for index, hit := range result.Hits {
		if hit.WebformatURL == "" {
			continue
		}
		candidates = append(candidates, model.ImageCandidate{Index: index, ID: hit.ID, Tags: hit.Tags, Creator: hit.User, Width: hit.ImageWidth, Height: hit.ImageHeight, Likes: hit.Likes, Downloads: hit.Downloads, ImageURL: hit.WebformatURL})
	}
	if len(candidates) == 0 {
		return nil, errors.New("Pixabay returned no downloadable images")
	}
	p.logger.Info("Pixabay image candidates found", "operation", "search_images", "provider", "pixabay", "query", query, "candidates", len(candidates))
	return candidates, nil
}

func (p *Providers) Download(ctx context.Context, candidate model.ImageCandidate) ([]byte, string, error) {
	if candidate.ImageURL == "" {
		return nil, "", errors.New("Pixabay candidate has no image URL")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, candidate.ImageURL, nil)
	if err != nil {
		return nil, "", err
	}
	response, err := p.client.Do(request)
	if err != nil {
		return nil, "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("Pixabay image host returned %s", response.Status)
	}
	contentType := response.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		return nil, "", fmt.Errorf("Pixabay image host returned unexpected content type %q", contentType)
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, maxResponseSize+1))
	if err != nil || len(content) > maxResponseSize {
		return nil, "", errors.New("Pixabay image exceeds size limit")
	}
	extension := filepath.Ext(request.URL.Path)
	if extension == "" {
		extensions, _ := mime.ExtensionsByType(contentType)
		if len(extensions) > 0 {
			extension = extensions[0]
		}
	}
	p.logger.Info("Pixabay image downloaded", "operation", "download_image", "provider", "pixabay", "candidateId", candidate.ID, "bytes", len(content))
	return content, extension, nil
}

func (p *Providers) GetDecks(ctx context.Context) ([]string, error) {
	var decks []string
	err := p.ankiCall(ctx, "deckNames", map[string]any{}, &decks)
	return decks, err
}

func (p *Providers) CreateNote(ctx context.Context, item model.QueueItem, deck string) (string, error) {
	if err := p.ensureAnkiModel(ctx); err != nil {
		return "", err
	}
	fields := map[string]string{
		"CardId": item.ID, "Word": html.EscapeString(item.Word), "Sentence": highlightStudyWord(item.Sentence, item.Word),
		"IPA": html.EscapeString(item.IPA), "Definition": html.EscapeString(item.Definition), "SentenceAudio": "", "Image": "",
	}
	if item.SentenceAudio != "" {
		if filename, err := p.storeMedia(ctx, item.SentenceAudio); err == nil {
			fields["SentenceAudio"] = "[sound:" + filename + "]"
		} else {
			return "", err
		}
	}
	if item.Image != nil {
		if filename, err := p.storeMedia(ctx, *item.Image); err == nil {
			fields["Image"] = `<img src="` + html.EscapeString(filename) + `">`
		} else {
			return "", err
		}
	}
	var noteID int64
	err := p.ankiCall(ctx, "addNote", map[string]any{"note": map[string]any{
		"deckName": deck, "modelName": "Anki Vocabulary", "fields": fields,
		"options": map[string]any{"allowDuplicate": false}, "tags": []string{"anki-vocabulary-manager"},
	}}, &noteID)
	return strconv.FormatInt(noteID, 10), err
}

func (p *Providers) ensureAnkiModel(ctx context.Context) error {
	p.ankiModelMu.Lock()
	defer p.ankiModelMu.Unlock()
	if p.ankiModelReady {
		return nil
	}
	var modelNames []string
	if err := p.ankiCall(ctx, "modelNames", map[string]any{}, &modelNames); err != nil {
		return err
	}
	for _, name := range modelNames {
		if name == "Anki Vocabulary" {
			p.ankiModelReady = true
			return nil
		}
	}
	params := map[string]any{
		"modelName":     "Anki Vocabulary",
		"inOrderFields": []string{"CardId", "Word", "Sentence", "IPA", "Definition", "SentenceAudio", "Image"},
		"css": `.card { font-family: Arial, sans-serif; font-size: 20px; text-align: center; color: #18211d; background: #fff; }
.sentence { font-size: 25px; line-height: 1.45; margin: 18px 0; }
.word { color: #2563eb; font-size: 34px; font-weight: 700; margin: 16px 0 8px; }
.ipa { color: #66716c; font-size: 20px; margin-bottom: 16px; }
.definition { font-size: 21px; line-height: 1.45; margin: 16px auto; max-width: 680px; }
.audio { margin: 16px 0; }
.image img { max-width: 100%; max-height: 360px; margin-top: 18px; border-radius: 6px; }`,
		"isCloze": false,
		"cardTemplates": []map[string]string{{
			"Name":  "Vocabulary",
			"Front": `<div class="sentence">{{Sentence}}</div><div class="audio">{{SentenceAudio}}</div>`,
			"Back":  `{{FrontSide}}<hr id="answer"><div class="word">{{Word}}</div><div class="ipa">{{IPA}}</div><div class="definition">{{Definition}}</div><div class="image">{{Image}}</div>`,
		}},
	}
	var created any
	if err := p.ankiCall(ctx, "createModel", params, &created); err != nil {
		return fmt.Errorf("create Anki Vocabulary model: %w", err)
	}
	p.ankiModelReady = true
	p.logger.Info("Anki note model created", "operation", "create_model", "provider", "ankiconnect", "model", "Anki Vocabulary")
	return nil
}

func highlightStudyWord(sentence, word string) string {
	terms := strings.Fields(word)
	if len(terms) == 0 {
		return html.EscapeString(sentence)
	}
	parts := make([]string, 0, len(terms))
	for _, term := range terms {
		parts = append(parts, regexp.QuoteMeta(term)+`[A-Za-z]*`)
	}
	pattern, err := regexp.Compile(`(?i)\b` + strings.Join(parts, `\s+`) + `\b`)
	if err != nil {
		return html.EscapeString(sentence)
	}
	match := pattern.FindStringIndex(sentence)
	if match == nil {
		return html.EscapeString(sentence)
	}
	return html.EscapeString(sentence[:match[0]]) + `<span style="color:#2563eb;font-weight:700">` + html.EscapeString(sentence[match[0]:match[1]]) + `</span>` + html.EscapeString(sentence[match[1]:])
}

func (p *Providers) storeMedia(ctx context.Context, path string) (string, error) {
	normalized := filepath.ToSlash(filepath.Clean(path))
	if !strings.HasPrefix(normalized, "storage/") {
		return "", errors.New("invalid media path")
	}
	relative := strings.TrimPrefix(normalized, "storage/")
	fullPath := filepath.Join(p.storageDir, filepath.FromSlash(relative))
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("read media: %w", err)
	}
	if len(content) > maxResponseSize {
		return "", errors.New("media exceeds size limit")
	}
	filename := filepath.Base(path)
	var stored string
	err = p.ankiCall(ctx, "storeMediaFile", map[string]any{"filename": filename, "data": base64.StdEncoding.EncodeToString(content)}, &stored)
	return stored, err
}

func (p *Providers) ankiCall(ctx context.Context, action string, params map[string]any, target any) error {
	body, _ := json.Marshal(map[string]any{"action": action, "version": 6, "params": params})
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  any             `json:"error"`
	}
	if err := p.doJSON(ctx, http.MethodPost, p.ankiURL, nil, body, &envelope); err != nil {
		return err
	}
	if envelope.Error != nil {
		return fmt.Errorf("AnkiConnect: %v", envelope.Error)
	}
	return json.Unmarshal(envelope.Result, target)
}

func (p *Providers) doJSON(ctx context.Context, method, endpoint string, headers map[string]string, body []byte, target any) error {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, err := p.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("request returned %s", response.Status)
	}
	return json.NewDecoder(io.LimitReader(response.Body, maxResponseSize)).Decode(target)
}

func (p *Providers) doJSONWithRetry(ctx context.Context, method, endpoint string, headers map[string]string, body []byte, target any) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		lastErr = p.doJSON(ctx, method, endpoint, headers, body, target)
		if lastErr == nil {
			return nil
		}
		if attempt < 2 {
			timer := time.NewTimer(time.Duration(attempt+1) * 250 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	return lastErr
}
