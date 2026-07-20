"use client";

import { ArrowUpRight, ShoppingBag } from "lucide-react";
import type { Room } from "@/lib/property";
import { STYLES } from "@/lib/property";
import { useStyle } from "./style-context";

export function ShopTheLook({ room }: { room: Room }) {
  const { style } = useStyle();
  const products = room.shop[style];
  const styleName = STYLES.find((s) => s.id === style)?.name ?? "";

  return (
    <div className="mt-6">
      <div className="mb-3 flex items-center gap-2">
        <ShoppingBag size={15} style={{ color: "var(--brand)" }} />
        <span className="eyebrow text-muted">Shop the {styleName} look</span>
      </div>

      {products ? (
        <ul className="grid grid-cols-2 gap-2.5 lg:grid-cols-4">
          {products.map((p) => (
            <li key={p.name}>
              <a
                href={p.url}
                target="_blank"
                rel="noopener noreferrer"
                className="flex h-full flex-col justify-between gap-3 rounded-xl border border-line bg-surface p-4 transition-colors duration-200 hover:border-[color:var(--brand)] cursor-pointer"
              >
                <span className="text-sm font-medium leading-snug text-ink">{p.name}</span>
                <span className="flex items-center justify-between">
                  <span className="flex flex-col">
                    <span className="eyebrow text-[0.6rem] text-muted">{p.retailer}</span>
                    <span className="font-mono text-sm tabular-nums text-ink">{p.price}</span>
                  </span>
                  <ArrowUpRight size={16} className="text-muted transition-colors group-hover:text-ink" style={{ color: "var(--brand)" }} />
                </span>
              </a>
            </li>
          ))}
        </ul>
      ) : (
        <p className="rounded-xl border border-dashed border-line bg-surface/50 px-4 py-5 text-sm text-muted">
          A curated shopping list for the {styleName} look is on the way. The Mediterranean edit is
          live now — switch styles to browse it.
        </p>
      )}
    </div>
  );
}
