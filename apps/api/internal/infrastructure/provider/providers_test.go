package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/local/anki-vocabulary/apps/api/internal/domain/model"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestGenerateDefinitionIncludesNormalizedSynonyms(t *testing.T) {
	providers := New(slog.Default(), "https://hive.local/api/v3", "deepseek-key", "deepseek-model", "", "", "", "", "")
	providers.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var payload struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.Messages) != 2 || !strings.Contains(payload.Messages[1].Content, "2-3 sentences") || !strings.Contains(payload.Messages[1].Content, "synonyms") {
			t.Fatalf("unexpected definition prompt: %#v", payload.Messages)
		}
		body := `{"choices":[{"message":{"content":"{\"definition\":\"Possible to do successfully. It often describes a practical plan that can work with available resources\",\"synonyms\":[\"possible\",\"practical\",\"feasible\",\"possible\",\"workable\",\"viable\",\"achievable\"]}"}}]}`
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
	})}

	definition, err := providers.GenerateDefinition(context.Background(), "feasible")
	if err != nil {
		t.Fatal(err)
	}
	expected := "Possible to do successfully. It often describes a practical plan that can work with available resources. Synonyms: possible, practical, workable, viable, achievable."
	if definition != expected {
		t.Fatalf("unexpected definition:\nwant: %s\n got: %s", expected, definition)
	}
}

func TestCreateNoteCreatesMissingAnkiModelOnce(t *testing.T) {
	providers := New(slog.Default(), "", "", "", "", "", "http://anki.local", "", "")
	actions := []string{}
	providers.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var payload struct {
			Action string         `json:"action"`
			Params map[string]any `json:"params"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		actions = append(actions, payload.Action)
		responseBody := `{"result":null,"error":null}`
		switch payload.Action {
		case "modelNames":
			responseBody = `{"result":[],"error":null}`
		case "createModel":
			fields, ok := payload.Params["inOrderFields"].([]any)
			if !ok || len(fields) != 7 || fields[0] != "CardId" || fields[6] != "Image" {
				t.Fatalf("unexpected model fields: %#v", payload.Params["inOrderFields"])
			}
			templates, ok := payload.Params["cardTemplates"].([]any)
			if !ok || len(templates) != 1 {
				t.Fatalf("unexpected card templates: %#v", payload.Params["cardTemplates"])
			}
			responseBody = `{"result":{"name":"Anki Vocabulary"},"error":null}`
		case "addNote":
			note, ok := payload.Params["note"].(map[string]any)
			if !ok || note["modelName"] != "Anki Vocabulary" {
				t.Fatalf("unexpected note payload: %#v", payload.Params["note"])
			}
			responseBody = `{"result":42,"error":null}`
		default:
			t.Fatalf("unexpected AnkiConnect action: %s", payload.Action)
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(responseBody)), Request: request}, nil
	})}

	item := model.QueueItem{ID: "card_test", Word: "reliable", Sentence: "It is reliable.", IPA: "/rɪˈlaɪəbəl/", Definition: "consistently good"}
	for range 2 {
		noteID, err := providers.CreateNote(context.Background(), item, "English")
		if err != nil || noteID != "42" {
			t.Fatalf("create note: %q, %v", noteID, err)
		}
	}
	if actual := strings.Join(actions, ","); actual != "modelNames,createModel,addNote,addNote" {
		t.Fatalf("unexpected AnkiConnect actions: %s", actual)
	}
}

func TestPixabaySearchReturnsMetadataAndDownloadsChosenCandidate(t *testing.T) {
	providers := New(slog.Default(), "", "", "", "", "", "", "", "pixabay-key")
	providers.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host == "pixabay.com" {
			if request.URL.Query().Get("q") != "streamline workflow" || request.URL.Query().Get("per_page") != "5" {
				t.Fatalf("unexpected Pixabay query: %s", request.URL.RawQuery)
			}
			return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"hits":[{"id":11,"tags":"traffic, road","user":"wrong","webformatURL":"https://cdn.example/11.jpg","imageWidth":640,"imageHeight":426,"likes":2,"downloads":8},{"id":22,"tags":"efficient workflow, process","user":"right","webformatURL":"https://cdn.example/22.jpg","imageWidth":960,"imageHeight":640,"likes":20,"downloads":80}]}`)), Request: request}, nil
		}
		if request.URL.String() != "https://cdn.example/22.jpg" {
			t.Fatalf("downloaded unselected candidate: %s", request.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: http.Header{"Content-Type": []string{"image/jpeg"}}, Body: io.NopCloser(strings.NewReader("chosen-image")), Request: request}, nil
	})}

	candidates, err := providers.Search(context.Background(), "streamline workflow", 5)
	if err != nil || len(candidates) != 2 || candidates[1].ID != 22 || candidates[1].Tags != "efficient workflow, process" {
		t.Fatalf("unexpected candidates: %#v, %v", candidates, err)
	}
	content, extension, err := providers.Download(context.Background(), candidates[1])
	if err != nil || string(content) != "chosen-image" || extension != ".jpg" {
		t.Fatalf("unexpected selected download: %q, %q, %v", content, extension, err)
	}
}

func TestElevenLabsGenerateLogsDiagnosticsWithoutSecrets(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	providers := New(logger, "", "", "", "secret-api-key", "voice-id", "", "", "")
	providers.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    http.StatusOK,
			Status:        "200 OK",
			Header:        http.Header{"Request-Id": []string{"request-123"}, "Content-Type": []string{"audio/mpeg"}},
			Body:          io.NopCloser(strings.NewReader("audio-data")),
			ContentLength: 10,
			Request:       request,
		}, nil
	})}

	audio, err := providers.Generate(context.Background(), "private sentence content")
	if err != nil {
		t.Fatal(err)
	}
	if string(audio) != "audio-data" {
		t.Fatalf("unexpected audio: %q", audio)
	}

	output := logs.String()
	for _, expected := range []string{"ElevenLabs audio generation started", "ElevenLabs response received", "ElevenLabs audio generation completed", "request-123", `"bytes":10`} {
		if !strings.Contains(output, expected) {
			t.Errorf("expected log output to contain %q", expected)
		}
	}
	for _, secret := range []string{"secret-api-key", "private sentence content"} {
		if strings.Contains(output, secret) {
			t.Errorf("log output exposed %q", secret)
		}
	}
}

func TestElevenLabsGenerateLogsRejectedResponse(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	providers := New(logger, "", "", "", "secret-api-key", "voice-id", "", "", "")
	providers.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Status:     "429 Too Many Requests",
			Header:     http.Header{"Request-Id": []string{"request-429"}},
			Body:       io.NopCloser(strings.NewReader(`{"detail":"quota exceeded"}`)),
			Request:    request,
		}, nil
	})}

	_, err := providers.Generate(context.Background(), "private sentence content")
	if err == nil || !strings.Contains(err.Error(), "429 Too Many Requests") {
		t.Fatalf("expected status error, got %v", err)
	}

	output := logs.String()
	for _, expected := range []string{"ElevenLabs audio generation rejected", "request-429", "quota exceeded"} {
		if !strings.Contains(output, expected) {
			t.Errorf("expected log output to contain %q", expected)
		}
	}
	for _, secret := range []string{"secret-api-key", "private sentence content"} {
		if strings.Contains(output, secret) {
			t.Errorf("log output exposed %q", secret)
		}
	}
}

func TestHighlightStudyWord(t *testing.T) {
	tests := []struct {
		name     string
		sentence string
		word     string
		expected string
	}{
		{
			name:     "inflected word",
			sentence: "The company streamlined its <process>.",
			word:     "streamline",
			expected: `The company <span style="color:#2563eb;font-weight:700">streamlined</span> its &lt;process&gt;.`,
		},
		{
			name:     "phrasal verb",
			sentence: "We planned ahead for tomorrow.",
			word:     "plan ahead",
			expected: `We <span style="color:#2563eb;font-weight:700">planned ahead</span> for tomorrow.`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if actual := highlightStudyWord(test.sentence, test.word); actual != test.expected {
				t.Fatalf("unexpected highlight:\nwant: %s\n got: %s", test.expected, actual)
			}
		})
	}
}
