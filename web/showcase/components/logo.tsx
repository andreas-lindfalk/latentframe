// Provisional Latent Frame mark — a geometric LF "frame" monogram, rendered so the
// strokes follow the theme (currentColor) and the accent square adopts the active
// style accent (var(--brand)). Swap for the final asset when it lands.
export function Logo({ className, label = true }: { className?: string; label?: boolean }) {
  return (
    <span className={`inline-flex items-center gap-2.5 ${className ?? ""}`}>
      <svg width="24" height="24" viewBox="0 0 100 100" fill="none" aria-hidden="true">
        <path d="M23 21h15v59H23z" fill="currentColor" />
        <path d="M38 21h40v13H38z" fill="currentColor" />
        <path d="M38 44h26v12H38z" fill="currentColor" />
        <path d="M23 67h55v13H23z" fill="currentColor" />
        <rect x="60" y="45" width="12" height="12" fill="var(--brand)" />
      </svg>
      {label && <span className="eyebrow">Latent&nbsp;Frame</span>}
    </span>
  );
}
