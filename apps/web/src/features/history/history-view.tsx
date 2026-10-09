"use client";

import { useQuery } from "@tanstack/react-query";
import { Archive, Search } from "lucide-react";
import { useDeferredValue, useState } from "react";

import { PageHeader } from "@/components/ui/page-header";
import { api } from "@/lib/api/client";

export function HistoryView() {
  const history = useQuery({ queryKey: ["history"], queryFn: api.history });
  const [search, setSearch] = useState("");
  const [deck, setDeck] = useState("ALL");
  const [date, setDate] = useState("");
  const deferredSearch = useDeferredValue(search.toLowerCase());
  const decks = [...new Set(history.data?.map((item) => item.deck).filter(Boolean))];
  const filtered = history.data?.filter((item) => (item.word.toLowerCase().includes(deferredSearch) || item.sentence.toLowerCase().includes(deferredSearch)) && (deck === "ALL" || item.deck === deck) && (!date || item.sentAt?.startsWith(date))) ?? [];

  return <><PageHeader eyebrow="Archive" title="Vocabulary history." description="Your local record remains the source of truth, independent of the Anki collection." /><section className="history-filters panel"><label className="search-field"><Search size={17} /><input className="filter-input" value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Search vocabulary" /></label><select className="deck-select" value={deck} onChange={(event) => setDeck(event.target.value)}><option value="ALL">All decks</option>{decks.map((name) => <option key={name}>{name}</option>)}</select><input className="filter-input" type="date" value={date} onChange={(event) => setDate(event.target.value)} aria-label="Filter by date" /></section>{!history.isLoading && !filtered.length ? <div className="panel empty-state"><Archive size={30} /><h2>No matching cards</h2><p>Sent vocabulary will be retained here.</p></div> : <div className="history-table"><div className="history-head"><span>Word</span><span>Sentence</span><span>Deck</span><span>Sent</span></div>{filtered.map((item) => <article className="history-row" key={item.id}><strong>{item.word}<small>{item.ipa}</small></strong><span>{item.sentence}</span><span>{item.deck}</span><time>{item.sentAt ? new Intl.DateTimeFormat("en", { dateStyle: "medium" }).format(new Date(item.sentAt)) : "-"}</time></article>)}</div>}</>;
}