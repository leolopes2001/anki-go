export function PageHeader({ eyebrow, title, description }: { eyebrow?: string; title: string; description: string }) {
  return (
    <header className="page-header">
      {eyebrow && <span className="eyebrow">{eyebrow}</span>}
      <h1>{title}</h1>
      <p>{description}</p>
    </header>
  );
}