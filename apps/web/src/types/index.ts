export type PackageStatus =
  | "OPEN"
  | "GENERATING_SENTENCES"
  | "REVIEWING"
  | "COMPLETED";

export type QueueStatus =
  | "PENDING"
  | "PROCESSING"
  | "READY"
  | "SENDING_TO_ANKI"
  | "SENT"
  | "ERROR";

export interface PackageItem {
  word: string;
  sentence?: string;
}

export interface VocabularyPackage {
  id: string;
  status: PackageStatus;
  items: PackageItem[];
  createdAt: string;
  updatedAt: string;
}

export interface QueueItem {
  id: string;
  packageId: string;
  word: string;
  sentence: string;
  ipa?: string;
  definition?: string;
  sentenceAudio?: string;
  image?: string | null;
  imageRevision?: string;
  imageSourceId?: number;
  status: QueueStatus;
  progress?: string;
  error?: string;
  attempts: number;
  createdAt: string;
  processedAt?: string;
  sentAt?: string;
  deck?: string;
  ankiNoteId?: string;
}

export interface Health {
  api: string;
  deepseek: string;
  elevenlabs: string;
  anki: string;
}

export interface Settings {
  ai: { provider: string; status: string };
  audio: { provider: string; status: string };
  images: { provider: string; status: string };
  anki: { provider: string; status: string };
}