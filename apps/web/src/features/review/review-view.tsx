"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowRight, BookOpen, Check, LoaderCircle, RefreshCw } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";

import { PageHeader } from "@/components/ui/page-header";
import { api } from "@/lib/api/client";
import type { PackageItem, VocabularyPackage } from "@/types";

export function ReviewView() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const current = useQuery({ queryKey: ["current-package"], queryFn: api.currentPackage });
  const confirm = useMutation({
    mutationFn: () => api.confirmReview(current.data!.id),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["current-package"] }),
        queryClient.invalidateQueries({ queryKey: ["queue"] }),
      ]);
      router.push("/queue");
    },
  });

  if (current.isLoading) return <ReviewSkeleton />;
  if (!current.data || current.data.status !== "REVIEWING") {
    return <><PageHeader eyebrow="Review" title="Review every sentence." description="Approve natural context before any card enters processing." /><div className="panel empty-state"><BookOpen size={30} /><h2>No package in review</h2><p>Finalize your current package to generate its sentences.</p><Link className="primary-button" href="/package">Open package <ArrowRight size={17} /></Link></div></>;
  }

  return <><PageHeader eyebrow={`${current.data.items.length} sentences`} title="Review every sentence." description="Regenerate only what needs work. Each accepted sentence is persisted automatically." /><div className="review-list">{current.data.items.map((item, index) => <SentenceCard key={item.word} packageId={current.data!.id} item={item} index={index} />)}</div>{confirm.error && <p className="error-banner">{confirm.error.message}</p>}<div className="review-confirm"><div><Check size={18} /><span><strong>Everything reads naturally?</strong><small>Cards will enter the processing queue.</small></span></div><button className="primary-button" onClick={() => confirm.mutate()} disabled={confirm.isPending}>{confirm.isPending ? <LoaderCircle className="spin" size={17} /> : <ArrowRight size={17} />} Confirm {current.data.items.length} sentences</button></div></>;
}

function SentenceCard({ packageId, item, index }: { packageId: string; item: PackageItem; index: number }) {
  const queryClient = useQueryClient();
  const regenerate = useMutation({
    mutationFn: () => api.regenerateSentence(packageId, item.word),
    onSuccess: (data) => queryClient.setQueryData<VocabularyPackage>(["current-package"], data),
  });
  return <article className="panel sentence-card"><div className="sentence-index">{String(index + 1).padStart(2, "0")}</div><div className="sentence-copy"><h2>{item.word}</h2>{regenerate.isPending ? <div className="sentence-loading"><div className="skeleton" /><div className="skeleton short" /></div> : <p>{item.sentence}</p>}{regenerate.error && <small className="inline-error">{regenerate.error.message}</small>}</div><button className="secondary-button regenerate-button" onClick={() => regenerate.mutate()} disabled={regenerate.isPending}>{regenerate.isPending ? <LoaderCircle className="spin" size={16} /> : <RefreshCw size={16} />} <span>{regenerate.isPending ? "Generating" : "New sentence"}</span></button></article>;
}

function ReviewSkeleton() {
  return <><PageHeader eyebrow="Review" title="Review every sentence." description="Loading your persisted review." /><div className="review-list">{Array.from({ length: 3 }).map((_, index) => <div className="panel sentence-card" key={index}><div className="skeleton review-skeleton" /></div>)}</div></>;
}