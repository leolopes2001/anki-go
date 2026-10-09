"use client";

import { useQuery } from "@tanstack/react-query";
import {
  Archive,
  BookOpen,
  Boxes,
  CircleGauge,
  LibraryBig,
  Menu,
  Settings,
  Sparkles,
  X,
} from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";

import { api } from "@/lib/api/client";

const navigation = [
  { href: "/", label: "Dashboard", icon: CircleGauge },
  { href: "/discover", label: "Discover", icon: Sparkles },
  { href: "/package", label: "Package", icon: Boxes },
  { href: "/review", label: "Review", icon: BookOpen },
  { href: "/queue", label: "Queue", icon: LibraryBig },
  { href: "/history", label: "History", icon: Archive },
];

export function AppShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const [open, setOpen] = useState(false);
  const health = useQuery({ queryKey: ["health"], queryFn: api.health, refetchInterval: 30_000 });
  const queue = useQuery({ queryKey: ["queue-summary"], queryFn: api.queue, refetchInterval: 5_000 });
  const activeQueue = queue.data?.filter((item) => item.status !== "SENT").length ?? 0;

  return (
    <div className="app-frame">
      <aside className={open ? "sidebar sidebar-open" : "sidebar"}>
        <div className="brand-row">
          <div className="brand-mark">V</div>
          <div>
            <strong>Vocabulary</strong>
            <span>Manager</span>
          </div>
          <button className="icon-button sidebar-close" onClick={() => setOpen(false)} aria-label="Close navigation">
            <X size={18} />
          </button>
        </div>
        <nav className="nav-list" aria-label="Primary navigation">
          {navigation.map(({ href, label, icon: Icon }) => {
            const active = href === "/" ? pathname === href : pathname.startsWith(href);
            return (
              <Link key={href} href={href} className={active ? "nav-link active" : "nav-link"} onClick={() => setOpen(false)}>
                <Icon size={18} strokeWidth={1.8} />
                {label}
              </Link>
            );
          })}
        </nav>
        <Link href="/settings" className={pathname.startsWith("/settings") ? "nav-link active settings-link" : "nav-link settings-link"} onClick={() => setOpen(false)}>
          <Settings size={18} strokeWidth={1.8} /> Settings
        </Link>
      </aside>
      {open && <button className="sidebar-scrim" onClick={() => setOpen(false)} aria-label="Close navigation" />}
      <div className="workspace">
        <header className="topbar">
          <button className="icon-button mobile-menu" onClick={() => setOpen(true)} aria-label="Open navigation">
            <Menu size={20} />
          </button>
          <span className="topbar-title">Vocabulary Manager</span>
          <div className="topbar-status">
            <span className={health.data?.anki === "connected" ? "status-dot online" : "status-dot"} />
            Anki: {health.data?.anki === "connected" ? "Connected" : "Offline"}
            <span className="queue-count">Queue {activeQueue}</span>
          </div>
        </header>
        <main className="content">{children}</main>
      </div>
    </div>
  );
}