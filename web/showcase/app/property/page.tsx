import { FullShowcase } from "@/components/full-showcase";
import { PROPERTIES } from "@/lib/properties";

// Legacy route — kept pointing at the Zeniamar property for back-compat.
export default function Page() {
  return <FullShowcase data={PROPERTIES["zeniamar-v"]} />;
}
