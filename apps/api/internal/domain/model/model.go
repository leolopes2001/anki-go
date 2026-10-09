package model

import "time"

type PackageStatus string

const (
	PackageOpen                PackageStatus = "OPEN"
	PackageGeneratingSentences PackageStatus = "GENERATING_SENTENCES"
	PackageReviewing           PackageStatus = "REVIEWING"
	PackageCompleted           PackageStatus = "COMPLETED"
)

type QueueStatus string

const (
	QueuePending       QueueStatus = "PENDING"
	QueueProcessing    QueueStatus = "PROCESSING"
	QueueReady         QueueStatus = "READY"
	QueueSendingToAnki QueueStatus = "SENDING_TO_ANKI"
	QueueSent          QueueStatus = "SENT"
	QueueError         QueueStatus = "ERROR"
)

type WordRecord struct {
	Word        string    `json:"word"`
	Context     string    `json:"context,omitempty"`
	Level       string    `json:"level,omitempty"`
	Status      string    `json:"status"`
	PresentedAt time.Time `json:"presentedAt"`
}

type PackageItem struct {
	Word     string `json:"word"`
	Sentence string `json:"sentence,omitempty"`
}

type Package struct {
	ID        string        `json:"id"`
	Status    PackageStatus `json:"status"`
	Items     []PackageItem `json:"items"`
	CreatedAt time.Time     `json:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt"`
}

type QueueItem struct {
	ID            string      `json:"id"`
	PackageID     string      `json:"packageId"`
	Word          string      `json:"word"`
	Sentence      string      `json:"sentence"`
	IPA           string      `json:"ipa,omitempty"`
	Definition    string      `json:"definition,omitempty"`
	SentenceAudio string      `json:"sentenceAudio,omitempty"`
	Image         *string     `json:"image"`
	ImageRevision string      `json:"imageRevision,omitempty"`
	ImageSourceID int         `json:"imageSourceId,omitempty"`
	Status        QueueStatus `json:"status"`
	Progress      string      `json:"progress,omitempty"`
	Error         string      `json:"error,omitempty"`
	Attempts      int         `json:"attempts"`
	CreatedAt     time.Time   `json:"createdAt"`
	ProcessedAt   *time.Time  `json:"processedAt,omitempty"`
	SentAt        *time.Time  `json:"sentAt,omitempty"`
	Deck          string      `json:"deck,omitempty"`
	AnkiNoteID    string      `json:"ankiNoteId,omitempty"`
}

type HistoryItem struct {
	QueueItem
}

type ImageDecision struct {
	UseImage bool   `json:"useImage"`
	Query    string `json:"query"`
}

type ImageCandidate struct {
	Index     int    `json:"index"`
	ID        int    `json:"id"`
	Tags      string `json:"tags"`
	Creator   string `json:"creator"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Likes     int    `json:"likes"`
	Downloads int    `json:"downloads"`
	ImageURL  string `json:"-"`
}

type Health struct {
	API        string `json:"api"`
	DeepSeek   string `json:"deepseek"`
	ElevenLabs string `json:"elevenlabs"`
	Anki       string `json:"anki"`
}
