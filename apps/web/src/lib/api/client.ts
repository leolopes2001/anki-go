import type {
  Health,
  QueueItem,
  Settings,
  VocabularyPackage,
} from "@/types";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080/api/v1";

export function mediaURL(path: string, revision?: string): string {
  if (!path.startsWith("storage/")) return "";
  const relativePath = path.slice("storage/".length).split("/").map(encodeURIComponent).join("/");
  const version = revision ? `?v=${encodeURIComponent(revision)}` : "";
  return `${API_URL}/media/${relativePath}${version}`;
}

interface Envelope<T> {
  data: T;
  error: { code: string; message: string } | null;
}

export class APIError extends Error {
  constructor(
    public readonly code: string,
    message: string,
  ) {
    super(message);
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_URL}${path}`, {
    ...init,
    headers: { "Content-Type": "application/json", ...init?.headers },
  });
  const payload = (await response.json()) as Envelope<T>;
  if (!response.ok || payload.error) {
    throw new APIError(
      payload.error?.code ?? "REQUEST_FAILED",
      payload.error?.message ?? "The request could not be completed.",
    );
  }
  return payload.data;
}

export const api = {
  health: () => request<Health>("/health"),
  generateVocabulary: (context: string, level: string) =>
    request<string[]>("/vocabulary/generate", {
      method: "POST",
      body: JSON.stringify({ context, level }),
    }),
  currentPackage: () => request<VocabularyPackage | null>("/packages/current"),
  addWords: (words: string[]) =>
    request<VocabularyPackage>("/packages/current/words", {
      method: "POST",
      body: JSON.stringify({ words }),
    }),
  removeWord: (word: string) =>
    request<VocabularyPackage>(
      `/packages/current/words/${encodeURIComponent(word)}`,
      { method: "DELETE" },
    ),
  generateSentences: () =>
    request<VocabularyPackage>("/packages/current/generate-sentences", {
      method: "POST",
    }),
  review: (id: string) => request<VocabularyPackage>(`/packages/${id}/review`),
  regenerateSentence: (packageId: string, word: string) =>
    request<VocabularyPackage>(
      `/packages/${packageId}/items/${encodeURIComponent(word)}/regenerate-sentence`,
      { method: "POST" },
    ),
  confirmReview: (packageId: string) =>
    request<QueueItem[]>(`/packages/${packageId}/confirm`, { method: "POST" }),
  queue: () => request<QueueItem[]>("/queue"),
  retry: (id: string) =>
    request<{ retried: boolean }>(`/queue/${id}/retry`, { method: "POST" }),
  generateImage: (id: string) =>
    request<QueueItem>(`/queue/${id}/image`, { method: "POST" }),
  deleteQueueItem: (id: string) =>
    request<{ deleted: boolean }>(`/queue/${id}`, { method: "DELETE" }),
  history: () => request<QueueItem[]>("/history"),
  decks: () => request<string[]>("/anki/decks"),
  sendToAnki: (deck: string, cardIds: string[]) =>
    request<QueueItem[]>("/anki/send", {
      method: "POST",
      body: JSON.stringify({ deck, cardIds }),
    }),
  settings: () => request<Settings>("/settings"),
};