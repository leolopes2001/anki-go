package port

import (
	"context"

	"github.com/local/anki-vocabulary/apps/api/internal/domain/model"
)

type Repository interface {
	SeenWords(context.Context) ([]string, error)
	RecordPresented(context.Context, []model.WordRecord) error
	CurrentPackage(context.Context) (*model.Package, error)
	FindPackage(context.Context, string) (*model.Package, error)
	SavePackage(context.Context, *model.Package) error
	Enqueue(context.Context, []model.QueueItem) error
	Queue(context.Context) ([]model.QueueItem, error)
	ClaimPending(context.Context) (*model.QueueItem, error)
	SaveQueueItem(context.Context, *model.QueueItem) error
	DeleteQueueItem(context.Context, string) error
	AppendHistory(context.Context, model.HistoryItem) error
	History(context.Context) ([]model.HistoryItem, error)
}

type AIProvider interface {
	AIHealth(context.Context) error
	GenerateVocabulary(context.Context, string, string, int, []string) ([]string, error)
	GenerateSentence(context.Context, string, string) (string, error)
	GenerateDefinition(context.Context, string) (string, error)
	AnalyzeImage(context.Context, string) (model.ImageDecision, error)
	SelectImage(context.Context, string, []model.ImageCandidate) (int, error)
}

type TTSProvider interface {
	Configured() bool
	Generate(context.Context, string) ([]byte, error)
}

type PronunciationProvider interface {
	GetIPA(context.Context, string) (string, error)
}

type ImageProvider interface {
	ImageConfigured() bool
	Search(context.Context, string, int) ([]model.ImageCandidate, error)
	Download(context.Context, model.ImageCandidate) ([]byte, string, error)
}

type AnkiProvider interface {
	AnkiHealth(context.Context) error
	GetDecks(context.Context) ([]string, error)
	CreateNote(context.Context, model.QueueItem, string) (string, error)
}

type MediaStore interface {
	SaveAudio(string, []byte) (string, error)
	SaveImage(string, string, []byte) (string, error)
	Read(string, string) ([]byte, string, error)
}
