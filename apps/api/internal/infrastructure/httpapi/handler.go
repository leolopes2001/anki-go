package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/local/anki-vocabulary/apps/api/internal/application"
)

type Handler struct {
	service *application.Service
	logger  *slog.Logger
}

type response struct {
	Data  any       `json:"data"`
	Error *apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func New(service *application.Service, logger *slog.Logger) http.Handler {
	handler := &Handler{service: service, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", handler.health)
	mux.HandleFunc("POST /api/v1/vocabulary/generate", handler.generateVocabulary)
	mux.HandleFunc("GET /api/v1/packages/current", handler.currentPackage)
	mux.HandleFunc("POST /api/v1/packages/current/words", handler.addWords)
	mux.HandleFunc("DELETE /api/v1/packages/current/words/{word}", handler.removeWord)
	mux.HandleFunc("POST /api/v1/packages/current/generate-sentences", handler.generateSentences)
	mux.HandleFunc("GET /api/v1/packages/{id}/review", handler.review)
	mux.HandleFunc("POST /api/v1/packages/{id}/items/{word}/regenerate-sentence", handler.regenerateSentence)
	mux.HandleFunc("POST /api/v1/packages/{id}/confirm", handler.confirm)
	mux.HandleFunc("GET /api/v1/queue", handler.queue)
	mux.HandleFunc("DELETE /api/v1/queue/{id}", handler.deleteQueueItem)
	mux.HandleFunc("POST /api/v1/queue/{id}/retry", handler.retry)
	mux.HandleFunc("POST /api/v1/queue/{id}/image", handler.generateImage)
	mux.HandleFunc("GET /api/v1/media/{directory}/{name}", handler.media)
	mux.HandleFunc("GET /api/v1/history", handler.history)
	mux.HandleFunc("GET /api/v1/anki/status", handler.ankiStatus)
	mux.HandleFunc("GET /api/v1/anki/decks", handler.decks)
	mux.HandleFunc("POST /api/v1/anki/send", handler.sendToAnki)
	mux.HandleFunc("GET /api/v1/settings", handler.settings)
	mux.HandleFunc("PUT /api/v1/settings", handler.settingsReadOnly)
	mux.HandleFunc("GET /api/openapi.json", openAPI)
	mux.HandleFunc("GET /api/docs", docs)
	return handler.middleware(mux)
}

func (h *Handler) generateVocabulary(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Context string `json:"context"`
		Level   string `json:"level"`
	}
	if !decode(w, r, &input) {
		return
	}
	words, err := h.service.GenerateVocabulary(r.Context(), input.Context, input.Level)
	h.respond(w, words, err)
}

func (h *Handler) currentPackage(w http.ResponseWriter, r *http.Request) {
	current, err := h.service.CurrentPackage(r.Context())
	h.respond(w, current, err)
}

func (h *Handler) addWords(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Words []string `json:"words"`
	}
	if !decode(w, r, &input) {
		return
	}
	current, err := h.service.AddWords(r.Context(), input.Words)
	h.respond(w, current, err)
}

func (h *Handler) removeWord(w http.ResponseWriter, r *http.Request) {
	current, err := h.service.RemoveWord(r.Context(), r.PathValue("word"))
	h.respond(w, current, err)
}

func (h *Handler) generateSentences(w http.ResponseWriter, r *http.Request) {
	current, err := h.service.GenerateSentences(r.Context())
	h.respond(w, current, err)
}

func (h *Handler) review(w http.ResponseWriter, r *http.Request) {
	current, err := h.service.Review(r.Context(), r.PathValue("id"))
	h.respond(w, current, err)
}

func (h *Handler) regenerateSentence(w http.ResponseWriter, r *http.Request) {
	current, err := h.service.RegenerateSentence(r.Context(), r.PathValue("id"), r.PathValue("word"))
	h.respond(w, current, err)
}

func (h *Handler) confirm(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ConfirmReview(r.Context(), r.PathValue("id"))
	h.respond(w, items, err)
}

func (h *Handler) queue(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.Queue(r.Context())
	h.respond(w, items, err)
}

func (h *Handler) retry(w http.ResponseWriter, r *http.Request) {
	err := h.service.Retry(r.Context(), r.PathValue("id"))
	h.respond(w, map[string]bool{"retried": err == nil}, err)
}

func (h *Handler) deleteQueueItem(w http.ResponseWriter, r *http.Request) {
	err := h.service.DeleteQueueItem(r.Context(), r.PathValue("id"))
	h.respond(w, map[string]bool{"deleted": err == nil}, err)
}

func (h *Handler) generateImage(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.GenerateImage(r.Context(), r.PathValue("id"))
	h.respond(w, item, err)
}

func (h *Handler) media(w http.ResponseWriter, r *http.Request) {
	content, contentType, err := h.service.Media(r.PathValue("directory"), r.PathValue("name"))
	if err != nil {
		writeError(w, http.StatusNotFound, "MEDIA_NOT_FOUND", "Media file not found.")
		return
	}
	w.Header().Set("Content-Type", contentType)
	if r.PathValue("directory") == "images" {
		w.Header().Set("Cache-Control", "no-store")
	} else {
		w.Header().Set("Cache-Control", "private, max-age=3600")
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.History(r.Context())
	h.respond(w, items, err)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	h.respond(w, h.service.Health(ctx), nil)
}

func (h *Handler) ankiStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	health := h.service.Health(ctx)
	h.respond(w, map[string]string{"status": health.Anki}, nil)
}

func (h *Handler) decks(w http.ResponseWriter, r *http.Request) {
	decks, err := h.service.Decks(r.Context())
	h.respond(w, decks, err)
}

func (h *Handler) sendToAnki(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Deck    string   `json:"deck"`
		CardIDs []string `json:"cardIds"`
	}
	if !decode(w, r, &input) {
		return
	}
	items, err := h.service.SendToAnki(r.Context(), input.Deck, input.CardIDs)
	h.respond(w, items, err)
}

func (h *Handler) settings(w http.ResponseWriter, r *http.Request) {
	health := h.service.Health(r.Context())
	h.respond(w, map[string]any{
		"ai":     map[string]string{"provider": "deepseek", "status": health.DeepSeek},
		"audio":  map[string]string{"provider": "elevenlabs", "status": health.ElevenLabs},
		"images": map[string]string{"provider": "pixabay", "status": h.service.ImageStatus()},
		"anki":   map[string]string{"provider": "ankiconnect", "status": health.Anki},
	}, nil)
}

func (h *Handler) settingsReadOnly(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusBadRequest, "SETTINGS_ENV_ONLY", "Settings are managed through environment variables.")
}

func (h *Handler) respond(w http.ResponseWriter, data any, err error) {
	if err == nil {
		writeJSON(w, http.StatusOK, response{Data: data})
		return
	}
	var domainError *application.Error
	if errors.As(err, &domainError) {
		writeError(w, http.StatusBadRequest, domainError.Code, domainError.Message)
		return
	}
	h.logger.Error("request failed", "error", err)
	writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred.")
}

func (h *Handler) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			bytes := make([]byte, 8)
			_, _ = rand.Read(bytes)
			requestID = hex.EncodeToString(bytes)
		}
		w.Header().Set("X-Request-ID", requestID)
		origin := r.Header.Get("Origin")
		if origin == "http://localhost:3000" || origin == "http://127.0.0.1:3000" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Request-ID")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
		h.logger.Info("request completed", "operation", "http_request", "requestId", requestID, "method", r.Method, "path", r.URL.Path, "duration", time.Since(started))
	})
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Request body is invalid.")
		return false
	}
	return true
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, response{Error: &apiError{Code: code, Message: message}})
}

func writeJSON(w http.ResponseWriter, status int, payload response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func docs(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, `<!doctype html><html><head><title>Anki Vocabulary API</title><meta charset="utf-8"><script src="https://unpkg.com/@stoplight/elements/web-components.min.js"></script><link rel="stylesheet" href="https://unpkg.com/@stoplight/elements/styles.min.css"></head><body><elements-api apiDescriptionUrl="/api/openapi.json" router="hash" layout="responsive"></elements-api></body></html>`)
}

func openAPI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(strings.TrimSpace(openAPISpec)))
}

const openAPISpec = `{
  "openapi":"3.1.0",
  "info":{"title":"Anki Vocabulary Manager API","version":"1.0.0"},
  "servers":[{"url":"http://localhost:8080"}],
  "paths":{
    "/api/v1/health":{"get":{"summary":"Provider health","responses":{"200":{"description":"Health status"}}}},
    "/api/v1/vocabulary/generate":{"post":{"summary":"Generate 10 unseen words","responses":{"200":{"description":"Vocabulary batch"},"400":{"description":"Invalid input"}}}},
    "/api/v1/packages/current":{"get":{"summary":"Get current package","responses":{"200":{"description":"Current package"}}}},
    "/api/v1/packages/current/words":{"post":{"summary":"Add words to package","responses":{"200":{"description":"Updated package"}}}},
    "/api/v1/packages/current/words/{word}":{"delete":{"summary":"Remove a word","parameters":[{"name":"word","in":"path","required":true,"schema":{"type":"string"}}],"responses":{"200":{"description":"Updated package"}}}},
    "/api/v1/packages/current/generate-sentences":{"post":{"summary":"Finalize and generate sentences","responses":{"200":{"description":"Review package"}}}},
    "/api/v1/packages/{id}/review":{"get":{"summary":"Get persisted review","responses":{"200":{"description":"Review"}}}},
    "/api/v1/packages/{id}/items/{word}/regenerate-sentence":{"post":{"summary":"Regenerate one sentence","responses":{"200":{"description":"Updated review"}}}},
    "/api/v1/packages/{id}/confirm":{"post":{"summary":"Confirm review and enqueue cards","responses":{"200":{"description":"Queue items"}}}},
	"/api/v1/queue":{"get":{"summary":"List queue","responses":{"200":{"description":"Queue"}}}},
	"/api/v1/queue/{id}":{"delete":{"summary":"Delete queue item","responses":{"200":{"description":"Item deleted"}}}},
    "/api/v1/queue/{id}/retry":{"post":{"summary":"Retry failed item","responses":{"200":{"description":"Retry accepted"}}}},
	"/api/v1/queue/{id}/image":{"post":{"summary":"Generate a missing card image","responses":{"200":{"description":"Image generated"}}}},
	"/api/v1/media/{directory}/{name}":{"get":{"summary":"Read generated media","responses":{"200":{"description":"Media content"},"404":{"description":"Media not found"}}}},
    "/api/v1/history":{"get":{"summary":"List sent cards","responses":{"200":{"description":"History"}}}},
    "/api/v1/anki/decks":{"get":{"summary":"List Anki decks","responses":{"200":{"description":"Decks"}}}},
    "/api/v1/anki/send":{"post":{"summary":"Send ready cards to Anki","responses":{"200":{"description":"Sent cards"}}}},
    "/api/v1/settings":{"get":{"summary":"Get safe provider settings","responses":{"200":{"description":"Settings without secrets"}}}}
  }
}`
