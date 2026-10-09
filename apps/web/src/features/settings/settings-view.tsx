"use client";

import { useQuery } from "@tanstack/react-query";
import { Bot, CheckCircle2, Image, Link2, Volume2, XCircle } from "lucide-react";
import { PageHeader } from "@/components/ui/page-header";
import { api } from "@/lib/api/client";

export function SettingsView() {
  const settings = useQuery({ queryKey: ["settings"], queryFn: api.settings });
  const sections = settings.data ? [
    { title: "AI", label: "DeepSeek", icon: Bot, value: settings.data.ai.status },
    { title: "Audio", label: "ElevenLabs", icon: Volume2, value: settings.data.audio.status },
    { title: "Images", label: "Pixabay", icon: Image, value: settings.data.images.status },
    { title: "Anki", label: "AnkiConnect", icon: Link2, value: settings.data.anki.status },
  ] : [];
  return <><PageHeader eyebrow="Configuration" title="Provider settings." description="Credentials stay in your local environment and are never exposed to the browser." /><div className="settings-list">{sections.map(({ title, label, icon: Icon, value }) => { const ok = ["ok", "configured", "connected"].includes(value); return <section className="panel settings-row" key={title}><div className="settings-icon"><Icon size={20} /></div><div><span>{title}</span><h2>{label}</h2></div><div className={ok ? "provider-status ok" : "provider-status"}>{ok ? <CheckCircle2 size={17} /> : <XCircle size={17} />} {value.replaceAll("_", " ")}</div></section>; })}</div><p className="settings-note">Change provider URLs, models, and credentials in the root <code>.env</code> file, then restart the API.</p></>;
}