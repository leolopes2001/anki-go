"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AlertCircle, CheckCircle2, CircleDashed, Eye, ImagePlus, LibraryBig, LoaderCircle, RefreshCw, Send, Trash2, Volume2, X } from "lucide-react";
import { useEffect, useState } from "react";

import { PageHeader } from "@/components/ui/page-header";
import { api, mediaURL } from "@/lib/api/client";
import type { QueueItem } from "@/types";

export function QueueView() {
  const queryClient = useQueryClient();
  const [deck, setDeck] = useState("");
  const [preview, setPreview] = useState<QueueItem | null>(null);
  const queue = useQuery({ queryKey: ["queue"], queryFn: api.queue, refetchInterval: 2_500 });
  const decks = useQuery({ queryKey: ["decks"], queryFn: api.decks, retry: false });
  const send = useMutation({ mutationFn: () => api.sendToAnki(deck, queue.data?.filter((item) => item.status === "READY").map((item) => item.id) ?? []), onSuccess: async () => { setPreview(null); await Promise.all([queryClient.invalidateQueries({ queryKey: ["queue"] }), queryClient.invalidateQueries({ queryKey: ["queue-summary"] }), queryClient.invalidateQueries({ queryKey: ["history"] })]); } });
  const ready = queue.data?.filter((item) => item.status === "READY") ?? [];

  return <><PageHeader eyebrow="Processing" title="Your card queue." description="IPA, definitions, audio, and useful images are generated locally in the background." /><section className="queue-toolbar panel"><div><strong>{queue.data?.length ?? 0}</strong><span>cards total</span></div><label className="field"><span>Destination deck</span><select className="deck-select" value={deck} onChange={(event) => setDeck(event.target.value)}><option value="">Select a deck</option>{decks.data?.map((name) => <option key={name}>{name}</option>)}</select></label><button className="primary-button" disabled={!deck || ready.length === 0 || send.isPending} onClick={() => send.mutate()}>{send.isPending ? <LoaderCircle className="spin" size={17} /> : <Send size={17} />} Send {ready.length || "ready"} to Anki</button></section>{(decks.error || send.error) && <p className="error-banner">{decks.error?.message ?? send.error?.message}</p>}{!queue.isLoading && !queue.data?.length ? <div className="panel empty-state"><LibraryBig size={30} /><h2>The queue is clear</h2><p>Approved review items will appear here.</p></div> : <div className="queue-list">{queue.data?.map((item) => <QueueRow key={item.id} item={item} onPreview={() => setPreview(item)} onDeleted={() => preview?.id === item.id && setPreview(null)} onImageGenerated={(updated) => setPreview((current) => current?.id === updated.id ? updated : current)} />)}</div>}{preview && <CardPreview item={preview} onClose={() => setPreview(null)} />}</>;
}

function QueueRow({ item, onPreview, onDeleted, onImageGenerated }: { item: QueueItem; onPreview: () => void; onDeleted: () => void; onImageGenerated: (item: QueueItem) => void }) {
  const queryClient = useQueryClient();
  const retry = useMutation({ mutationFn: () => api.retry(item.id), onSuccess: () => queryClient.invalidateQueries({ queryKey: ["queue"] }) });
  const generateImage = useMutation({ mutationFn: () => api.generateImage(item.id), onSuccess: async (updated) => { onImageGenerated(updated); await queryClient.invalidateQueries({ queryKey: ["queue"] }); } });
  const remove = useMutation({ mutationFn: () => api.deleteQueueItem(item.id), onSuccess: async () => { onDeleted(); await Promise.all([queryClient.invalidateQueries({ queryKey: ["queue"] }), queryClient.invalidateQueries({ queryKey: ["queue-summary"] })]); } });
  const Icon = item.status === "READY" || item.status === "SENT" ? CheckCircle2 : item.status === "ERROR" ? AlertCircle : CircleDashed;
  const busy = item.status === "PROCESSING" || item.status === "SENDING_TO_ANKI";
  function deleteItem() {
    if (window.confirm(`Delete “${item.word}” from the queue?`)) remove.mutate();
  }
  const actionError = generateImage.error?.message || remove.error?.message;
  return <article className="panel queue-row"><div className={`queue-state ${item.status.toLowerCase()}`}><Icon size={20} className={item.status === "PROCESSING" ? "spin" : ""} /></div><div className="queue-copy"><div><h2>{item.word}</h2><span className={`status-badge ${item.status.toLowerCase()}`}>{item.status.replaceAll("_", " ")}</span></div><p>{item.sentence}</p><small className={item.status === "ERROR" || actionError ? "inline-error" : ""}>{actionError || item.error || item.progress || "Waiting to be processed"}</small></div><div className="queue-actions">{item.status === "READY" && <button className="secondary-button" onClick={() => generateImage.mutate()} disabled={generateImage.isPending}>{generateImage.isPending ? <LoaderCircle className="spin" size={16} /> : item.image ? <RefreshCw size={16} /> : <ImagePlus size={16} />} {generateImage.isPending ? "Finding image" : item.image ? "Refresh image" : "Add image"}</button>}{item.status === "READY" && <button className="secondary-button" onClick={onPreview}><Eye size={16} /> Preview</button>}{item.status === "ERROR" && <button className="secondary-button" onClick={() => retry.mutate()} disabled={retry.isPending}>{retry.isPending ? <LoaderCircle className="spin" size={16} /> : <RefreshCw size={16} />} Retry</button>}<button className="icon-button queue-delete" aria-label={`Delete ${item.word}`} title={busy ? "Wait for processing to finish" : `Delete ${item.word}`} onClick={deleteItem} disabled={busy || remove.isPending}>{remove.isPending ? <LoaderCircle className="spin" size={16} /> : <Trash2 size={16} />}</button></div></article>;
}

function CardPreview({ item, onClose }: { item: QueueItem; onClose: () => void }) {
  useEffect(() => {
    function closeOnEscape(event: KeyboardEvent) { if (event.key === "Escape") onClose(); }
    document.addEventListener("keydown", closeOnEscape);
    document.body.style.overflow = "hidden";
    return () => { document.removeEventListener("keydown", closeOnEscape); document.body.style.overflow = ""; };
  }, [onClose]);

  const image = item.image ? mediaURL(item.image, item.imageRevision) : "";
  const audio = item.sentenceAudio ? mediaURL(item.sentenceAudio) : "";
  return <div className="preview-overlay" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}><section className="preview-dialog" role="dialog" aria-modal="true" aria-labelledby="preview-title"><header className="preview-header"><div><span>Ready for Anki</span><h2 id="preview-title">Card preview</h2></div><button className="icon-button" aria-label="Close preview" title="Close preview" onClick={onClose}><X size={18} /></button></header><div className="preview-body"><div className="preview-card"><p className="preview-meaning"><strong>{item.word}</strong> <span>{item.ipa || "IPA unavailable"}</span>: {item.definition || "Definition unavailable"}</p><p className="preview-sentence">{highlightStudyWord(item.sentence, item.word)}</p>{image && <div className="preview-image" role="img" aria-label={`Reference for ${item.word}`} style={{ backgroundImage: `url(${JSON.stringify(image)})` }} />}</div><div className="preview-audio"><Volume2 size={18} /><div><strong>Sentence audio</strong><span>{audio ? "Generated by ElevenLabs" : "Audio unavailable"}</span></div>{audio && <audio controls preload="metadata" src={audio}>Your browser does not support audio playback.</audio>}</div></div></section></div>;
}

function highlightStudyWord(sentence: string, word: string) {
  const terms = word.trim().split(/\s+/).filter(Boolean);
  if (!terms.length) return sentence;
  const pattern = terms.map((term) => `${term.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}[A-Za-z]*`).join("\\s+");
  const match = new RegExp(`\\b${pattern}\\b`, "i").exec(sentence);
  if (!match || match.index === undefined) return sentence;
  return <>{sentence.slice(0, match.index)}<strong className="study-word">{match[0]}</strong>{sentence.slice(match.index + match[0].length)}</>;
}