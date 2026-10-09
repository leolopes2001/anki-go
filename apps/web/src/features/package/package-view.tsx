"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowRight, Boxes, LoaderCircle, Trash2 } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { PageHeader } from "@/components/ui/page-header";
import { api } from "@/lib/api/client";

export function PackageView() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const current = useQuery({ queryKey: ["current-package"], queryFn: api.currentPackage });
  const remove = useMutation({ mutationFn: api.removeWord, onSuccess: (data) => queryClient.setQueryData(["current-package"], data) });
  const finalize = useMutation({ mutationFn: api.generateSentences, onSuccess: (data) => { queryClient.setQueryData(["current-package"], data); router.push("/review"); } });
  return <><PageHeader eyebrow="Package" title="Your working set." description="Keep this group focused. You can add more batches or remove words before generating sentences." />{current.isLoading ? <div className="panel package-list"><div className="skeleton loading-line" /><div className="skeleton loading-line" /></div> : !current.data?.items.length ? <div className="panel empty-state"><Boxes size={30} /><h2>Your package is empty</h2><p>Discover a batch and select the words you want to remember.</p><Link href="/discover" className="primary-button">Discover words <ArrowRight size={17} /></Link></div> : <section className="panel package-list"><div className="package-summary"><div><strong>{current.data.items.length}</strong><span>words in this package</span></div><Link href="/discover" className="secondary-button">Add more words</Link></div><div className="package-items">{current.data.items.map((item, index) => <div className="package-row" key={item.word}><span className="row-number">{String(index + 1).padStart(2, "0")}</span><strong>{item.word}</strong><button className="icon-button" aria-label={`Remove ${item.word}`} title={`Remove ${item.word}`} onClick={() => remove.mutate(item.word)} disabled={remove.isPending}><Trash2 size={16} /></button></div>)}</div>{(remove.error || finalize.error) && <p className="error-banner">{remove.error?.message ?? finalize.error?.message}</p>}<div className="package-action"><div><strong>Ready for context?</strong><span>One sentence will be generated for every word.</span></div><button className="primary-button" onClick={() => finalize.mutate()} disabled={finalize.isPending}>{finalize.isPending ? <LoaderCircle className="spin" size={17} /> : <ArrowRight size={17} />} {finalize.isPending ? "Generating sentences" : "Finalize & generate sentences"}</button></div></section>}</>;
}