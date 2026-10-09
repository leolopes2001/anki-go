"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Check, LoaderCircle, Plus, Sparkles } from "lucide-react";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { PageHeader } from "@/components/ui/page-header";
import { api } from "@/lib/api/client";

export function DiscoverView() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [directWord, setDirectWord] = useState("");
  const [context, setContext] = useState("Daily conversation");
  const [level, setLevel] = useState("B2");
  const [words, setWords] = useState<string[]>([]);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [notice, setNotice] = useState("");
  const generate = useMutation({ mutationFn: () => api.generateVocabulary(context, level), onSuccess: (data) => { setWords(data); setSelected(new Set()); setNotice(""); } });
  const add = useMutation({ mutationFn: () => api.addWords([...selected]), onSuccess: async () => { await queryClient.invalidateQueries({ queryKey: ["current-package"] }); setNotice(`${selected.size} ${selected.size === 1 ? "word" : "words"} added to your package.`); setSelected(new Set()); } });
  const addDirectly = useMutation({
    mutationFn: async () => {
      await api.addWords([directWord.trim()]);
      return api.generateSentences();
    },
    onSuccess: (data) => {
      queryClient.setQueryData(["current-package"], data);
      setDirectWord("");
      router.push("/review");
    },
  });
  function toggle(word: string) { setSelected((current) => { const next = new Set(current); if (next.has(word)) next.delete(word); else next.add(word); return next; }); }
  const error = generate.error?.message ?? add.error?.message ?? addDirectly.error?.message;
  return <><PageHeader eyebrow="Discover" title="What do you want to learn?" description="Describe a setting or subject. Every batch contains ten new words that have not appeared before." /><form className="panel direct-word-form" onSubmit={(event) => { event.preventDefault(); if (directWord.trim()) addDirectly.mutate(); }}><label className="field"><span>Add a word or expression directly</span><input value={directWord} maxLength={80} onChange={(event) => setDirectWord(event.target.value)} placeholder="e.g. figure out" autoComplete="off" /></label><button className="primary-button" type="submit" disabled={addDirectly.isPending || !directWord.trim()}>{addDirectly.isPending ? <LoaderCircle className="spin" size={18} /> : <Plus size={18} />} {addDirectly.isPending ? "Generating sentence" : "Add & review"}</button></form><div className="discover-divider"><span>or discover words by context</span></div><section className="panel discover-form"><label className="field context-field"><span>Context</span><input value={context} maxLength={120} onChange={(event) => setContext(event.target.value)} placeholder="e.g. Software development" /></label><label className="field level-field"><span>Level</span><select value={level} onChange={(event) => setLevel(event.target.value)}><option>B1</option><option>B2</option><option>C1</option></select></label><button className="primary-button generate-button" onClick={() => generate.mutate()} disabled={generate.isPending || !context.trim()}>{generate.isPending ? <LoaderCircle className="spin" size={18} /> : <Sparkles size={18} />} {generate.isPending ? "Generating" : words.length ? "Generate 10 more" : "Generate 10 words"}</button></section>{error && <p className="error-banner">{error}</p>}{notice && <div className="toast" role="status"><Check size={17} />{notice}</div>}{generate.isPending && <div className="word-grid">{Array.from({ length: 10 }).map((_, index) => <div className="word-card skeleton" key={index} />)}</div>}{!generate.isPending && words.length > 0 && <div className="word-grid">{words.map((word) => <button className={selected.has(word) ? "word-card selected" : "word-card"} key={word} onClick={() => toggle(word)}><span>{word}</span><span className="word-check">{selected.has(word) && <Check size={15} />}</span></button>)}</div>}{selected.size > 0 && <div className="selection-bar"><span><strong>{selected.size}</strong> selected</span><button className="primary-button" onClick={() => add.mutate()} disabled={add.isPending}>{add.isPending ? <LoaderCircle className="spin" size={17} /> : <Plus size={17} />} Add to package</button></div>}</>;
}