import Image from "next/image";
import Link from "next/link";
import { ArrowUpRight } from "lucide-react";
import { Logo } from "@/components/logo";
import { ThemeToggle } from "@/components/theme-toggle";
import { propertyCards } from "@/lib/properties";

// Landing = the property picker. Each card links to /p/<slug>.
export default function Home() {
  const cards = propertyCards();
  return (
    <div className="min-h-[100svh]">
      <header className="mx-auto flex w-full max-w-6xl items-center justify-between px-5 py-5 sm:px-8">
        <Logo />
        <ThemeToggle />
      </header>

      <main className="mx-auto w-full max-w-6xl px-5 pb-24 sm:px-8">
        <div className="pt-10 sm:pt-16">
          <p className="eyebrow mb-4" style={{ color: "var(--brand)" }}>
            The potential, honestly staged
          </p>
          <h1 className="max-w-[18ch] font-display text-[clamp(2.5rem,7vw,5rem)] font-light leading-[0.98] tracking-[-0.03em] text-balance">
            Every property, seen at its best.
          </h1>
          <p className="mt-6 max-w-[54ch] text-lg leading-relaxed text-muted">
            Restaged from the listing photos alone — same walls, same windows, only the possibility added.
            Choose a property.
          </p>
        </div>

        <div className="mt-12 grid gap-6 sm:grid-cols-2">
          {cards.map((c) => (
            <Link
              key={c.slug}
              href={`/p/${c.slug}`}
              className="group block overflow-hidden rounded-2xl border border-line bg-canvas transition-transform duration-200 hover:-translate-y-1"
            >
              <div className="relative aspect-[4/3] w-full overflow-hidden">
                <Image
                  src={c.img}
                  alt={c.property.name}
                  fill
                  sizes="(max-width: 640px) 100vw, 50vw"
                  className="object-cover transition-transform duration-500 group-hover:scale-105"
                />
              </div>
              <div className="flex items-start justify-between gap-4 p-5">
                <div>
                  <h2 className="font-display text-2xl font-light tracking-tight">{c.property.name}</h2>
                  <p className="mt-1 text-sm text-muted">
                    <span className="capitalize">{c.property.type}</span> · {c.property.location} · {c.spaceCount} spaces
                  </p>
                </div>
                <span
                  className="mt-1 inline-flex shrink-0 items-center gap-1 text-sm font-medium"
                  style={{ color: "var(--brand)" }}
                >
                  View
                  <ArrowUpRight size={16} className="transition-transform group-hover:translate-x-0.5 group-hover:-translate-y-0.5" />
                </span>
              </div>
            </Link>
          ))}
        </div>
      </main>
    </div>
  );
}
