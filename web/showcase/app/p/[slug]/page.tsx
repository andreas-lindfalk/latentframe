import { notFound } from "next/navigation";
import { FullShowcase } from "@/components/full-showcase";
import { PROPERTIES, PROPERTY_SLUGS } from "@/lib/properties";

// One route for every property: /p/<slug>. Add a property in lib/properties.ts.
export function generateStaticParams() {
  return PROPERTY_SLUGS.map((slug) => ({ slug }));
}

export default async function PropertyPage({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const data = PROPERTIES[slug];
  if (!data) notFound();
  return <FullShowcase data={data} />;
}
