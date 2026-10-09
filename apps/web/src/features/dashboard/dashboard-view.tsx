"use client";

import { useQuery } from "@tanstack/react-query";
import { ArrowRight, BookOpen, Boxes, CheckCircle2, LibraryBig } from "lucide-react";
import Link from "next/link";

import { PageHeader } from "@/components/ui/page-header";
import { api } from "@/lib/api/client";

export function DashboardView() {
  const currentPackage = useQuery({ queryKey: ["current-package"], queryFn: api.currentPackage });
  const queue = useQuery({ queryKey: ["queue"], queryFn: api.queue });
  const history = useQuery({ queryKey: ["history"], queryFn: api.history });
  const pending = queue.data?.filter((item) => item.status !== "SENT") ?? [];
  return <><PageHeader eyebrow="Your workspace" title="Build vocabulary that sticks." description="Move new English from discovery to useful, context-rich Anki cards." /><section className="metric-grid" aria-label="Vocabulary summary"><Metric icon={BookOpen} value={history.data?.length ?? 0} label="Words learned" tone="green" /><Metric icon={CheckCircle2} value={history.data?.filter((item) => item.status === "SENT").length ?? 0} label="Sent to Anki" tone="gold" /><Metric icon={Boxes} value={currentPackage.data?.items.length ?? 0} label="Current package" tone="blue" /><Metric icon={LibraryBig} value={pending.length} label="In the queue" tone="red" /></section><section className="dashboard-grid"><div className="panel dashboard-primary"><span className="section-kicker">Next session</span><h2>Find words worth remembering.</h2><p>Choose a context and level. Every suggestion is tracked so each batch stays fresh.</p><Link href="/discover" className="primary-button">Discover vocabulary <ArrowRight size={17} /></Link></div><div className="panel compact-list"><div className="section-heading"><div><span className="section-kicker">Current work</span><h2>Package</h2></div><Link href="/package">Open</Link></div>{currentPackage.data?.items.length ? currentPackage.data.items.slice(0, 5).map((item, index) => <div className="mini-row" key={item.word}><span>{String(index + 1).padStart(2, "0")}</span><strong>{item.word}</strong></div>) : <p className="muted-copy">No words selected yet.</p>}</div><div className="panel compact-list wide-panel"><div className="section-heading"><div><span className="section-kicker">Latest</span><h2>Recent vocabulary</h2></div><Link href="/history">View history</Link></div>{history.data?.length ? history.data.slice(-4).reverse().map((item) => <div className="recent-row" key={item.id}><strong>{item.word}</strong><span>{item.sentence}</span><small>{item.deck}</small></div>) : <p className="muted-copy">Sent cards will appear here.</p>}</div></section></>;
}

function Metric({ icon: Icon, value, label, tone }: { icon: typeof BookOpen; value: number; label: string; tone: string }) {
  return <div className="metric"><div className={`metric-icon ${tone}`}><Icon size={18} /></div><strong>{value.toLocaleString()}</strong><span>{label}</span></div>;
}