"use client";

import { ROOMS, PROPERTY, STYLES } from "@/lib/property";
import { useStyle } from "./style-context";
import { MorphImage } from "./morph-image";
import { BeforeAfter } from "./before-after";
import { ShopTheLook } from "./shop-the-look";
import { Reveal } from "./reveal";

export function RoomTour() {
  const { style } = useStyle();
  const styleName = STYLES.find((s) => s.id === style)?.name ?? "";

  return (
    <div className="mx-auto max-w-6xl px-5 sm:px-8">
      {ROOMS.map((room) => (
        <section key={room.id} className="border-t border-line py-16 sm:py-24">
          <Reveal className="mb-8 grid gap-x-10 gap-y-4 md:grid-cols-[auto_1fr] md:items-end">
            <div className="flex items-baseline gap-4">
              <span className="font-display text-4xl font-light tabular-nums sm:text-5xl" style={{ color: "var(--brand)" }}>
                {room.index}
              </span>
              <h2 className="font-display text-3xl font-light tracking-tight sm:text-4xl">{room.name}</h2>
            </div>
            <p className="max-w-[56ch] text-[15px] leading-relaxed text-muted md:justify-self-end md:text-right">
              {room.blurb}
            </p>
          </Reveal>

          <Reveal delay={0.05}>
            <BeforeAfter
              before={room.before}
              alt={`${PROPERTY.name} ${room.name.toLowerCase()}`}
              after={
                <MorphImage
                  srcKey={style}
                  src={room.after[style]}
                  alt={`${PROPERTY.name} ${room.name.toLowerCase()}, ${styleName} styling`}
                  sizes="(max-width: 900px) 100vw, 55vw"
                />
              }
            />
          </Reveal>

          <Reveal delay={0.1}>
            <ShopTheLook room={room} />
          </Reveal>
        </section>
      ))}
    </div>
  );
}
