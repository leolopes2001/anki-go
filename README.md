# Anki Vocabulary Manager

## About

Anki Vocabulary Manager is a local-first application for discovering English vocabulary and automatically creating enriched Anki cards. The workflow is intentionally explicit: **Discover -> Package -> Review -> Queue -> Anki**. Metadata, audio, images, and history remain on your machine.

## Features

- Generates batches of exactly 10 unseen words by context and CEFR level.
- Persists a working package and sentence review between restarts.
- Regenerates one sentence without blocking the rest of the review.
- Processes IPA, English definitions, sentence audio, and optional images in a local worker.
- Lists Anki decks, stores media, creates notes, and retains independent local history.
- Uses atomic JSON writes and validates media paths and HTTP input.

## Screenshots

Run the application to view the responsive Dashboard, Discover, Package, Review, Queue, History, and Settings screens at `http://localhost:3000`.

## Architecture

The Next.js application only calls the Go REST API. The API owns all integrations and storage. Domain contracts point inward; infrastructure implements JSON repositories and the DeepSeek through The Hive, ElevenLabs, Pixabay, and AnkiConnect providers.

```text
Browser -> Next.js :3000 -> Go API :8080
                              |-> DeepSeek via The Hive
                              |-> ElevenLabs
                              |-> Pixabay
                              |-> AnkiConnect
                              `-> data/ + storage/
```

## Project Structure

```text
apps/api/   Go API, domain, use cases, providers, worker
apps/web/   Next.js App Router frontend and feature modules
data/       Local JSON metadata (ignored by Git)
storage/    Generated audio and downloaded images (ignored by Git)
```

## Requirements

- Go 1.25 or newer (developed with Go 1.27.2)
- Node.js 20 or newer and npm 10 or newer
- The Hive API key with access to the configured DeepSeek model
- Pixabay API key for optional card images
- Anki Desktop with AnkiConnect
- ElevenLabs API key and voice ID for audio generation

## Installation

```bash
git clone <repository-url> anki-vocabulary
cd anki-vocabulary
cp .env.example .env
cd apps/api && go mod download && cd ../..
cd apps/web && npm install && cd ../..
```

## Configuration

Set `DEEPSEEK_API_KEY`, `ELEVENLABS_API_KEY`, `ELEVENLABS_VOICE_ID`, and `PIXABAY_API_KEY` in `.env`. Secrets are read only by the Go process and must never use the `NEXT_PUBLIC_` prefix. DeepSeek also generates the IPA pronunciation used in each card and selects the best Pixabay result from metadata before the API downloads it.

## Environment Variables

| Variable | Required | Description |
| --- | --- | --- |
| `API_PORT` | No | Go API port, default `8080` |
| `WEB_PORT` | No | Next.js port, default `3000` |
| `DATA_DIR` | No | JSON metadata directory |
| `STORAGE_DIR` | No | Audio and image directory |
| `AI_PROVIDER` | Yes | AI strategy; currently `deepseek` |
| `DEEPSEEK_URL` | Yes | The Hive API v3 base URL |
| `DEEPSEEK_API_KEY` | Yes | The Hive secret API key |
| `DEEPSEEK_MODEL` | Yes | DeepSeek model exposed by The Hive |
| `ELEVENLABS_API_KEY` | Yes* | TTS credential, required for queue processing |
| `ELEVENLABS_VOICE_ID` | Yes* | ElevenLabs voice |
| `PIXABAY_API_KEY` | Yes* | Pixabay credential, required for card images |
| `ANKI_CONNECT_URL` | Yes | AnkiConnect endpoint |
| `DEFAULT_ANKI_DECK` | No | Preferred destination deck |
| `QUEUE_WORKERS` | No | Local worker count, default `1` |
| `NEXT_PUBLIC_API_URL` | No | Browser-visible API base URL |

## Running the Project

```bash
make dev
```

The web app is available at `http://localhost:3000`, the API at `http://localhost:8080`, and interactive API documentation at `http://localhost:8080/api/docs`.

## Running in Development

Run processes separately when debugging:

```bash
make api
make web
```

## Production Build

```bash
make build
```

The API binary is written to `bin/anki-api`; start the generated Next.js build with `cd apps/web && npm start`.

## DeepSeek Setup

1. Create a The Hive API key with access to DeepSeek.
2. Set `DEEPSEEK_API_KEY` in `.env`.
3. Keep `DEEPSEEK_URL=https://api.thehive.ai/api/v3`.
4. Keep `DEEPSEEK_MODEL=deepseek-ai/deepseek-v4.1-flash`, or choose another supported model.

## ElevenLabs Setup

Create an API key and choose a voice in ElevenLabs, then set `ELEVENLABS_API_KEY` and `ELEVENLABS_VOICE_ID`. Keys are never returned by `/settings` or written to logs.

## AnkiConnect Setup

1. Install Anki Desktop.
2. Install add-on `2055492159` (AnkiConnect).
3. Start Anki and confirm port `8765` is available.
4. Open Queue, select a deck, and send READY cards. The API creates the `Anki Vocabulary` note type automatically when it is missing.

## Storage

`data/words.json`, `packages.json`, `queue.json`, and `history.json` are created automatically. Writes use a same-directory temporary file, `fsync`, and atomic rename. `storage/audio/` and `storage/images/` contain generated media. These paths are ignored by Git.

## Queue

The in-process worker claims one `PENDING` item atomically and advances it through `PROCESSING -> READY`. Failures become `ERROR` and can be retried from the UI. Sending uses `SENDING_TO_ANKI -> SENT`.

## API

All responses use `{ "data": ..., "error": null }` or `{ "data": null, "error": { "code": "...", "message": "..." } }`.

| Method | Endpoint | Purpose |
| --- | --- | --- |
| POST | `/api/v1/vocabulary/generate` | Generate 10 unseen words |
| GET/POST | `/api/v1/packages/current` | Read the current package / generate sentences |
| POST | `/api/v1/packages/current/words` | Add selected words |
| GET | `/api/v1/packages/:id/review` | Resume persisted review |
| POST | `/api/v1/packages/:id/items/:word/regenerate-sentence` | Regenerate one sentence |
| POST | `/api/v1/packages/:id/confirm` | Approve and enqueue cards |
| GET | `/api/v1/queue` | List processing state |
| GET | `/api/v1/anki/decks` | List destination decks |
| POST | `/api/v1/anki/send` | Send READY cards |
| GET | `/api/v1/history` | Read local history |

Example:

```bash
curl -X POST http://localhost:8080/api/v1/vocabulary/generate \
  -H 'Content-Type: application/json' \
  -d '{"context":"Software development","level":"B2"}'
```

## Card Lifecycle

```text
DISCOVERED -> SELECTED -> PACKAGE -> REVIEW -> PENDING
-> PROCESSING -> READY -> SENT
                    `-> ERROR -> PENDING (retry)
```

## Troubleshooting

**AnkiConnect unavailable:** verify Anki is running, AnkiConnect is installed, and port `8765` is free. The API status should report `anki: connected`.

**DeepSeek generation failed:** confirm the The Hive API key has access to `DEEPSEEK_MODEL` and available credits, then retry the operation.

**Audio generation failed:** confirm both ElevenLabs values are present and that the account has available quota, then retry the queue item.

## Development Guidelines

Run `make test` and `make lint` before committing. Keep domain interfaces small, do not expose provider credentials to the web app, validate every new input, and mock external providers in use-case tests.

## Roadmap

- Richer queue diagnostics and provider metrics
- Configurable Anki note templates
- Optional SQLite repository
- Screenshot automation and broader browser tests