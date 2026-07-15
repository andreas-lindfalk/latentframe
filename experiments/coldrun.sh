#!/usr/bin/env bash
# Cold-run the depth-t2i aesthetic engine across BOTH properties, every key room:
# best-of-N (director beststage) with the inspire honesty bar. This is the validation
# that upgrades "proven core" -> "reliable product core" — reliable wow at breadth.
#
# EVAL tooling only: it invokes the real Go pipeline (director beststage); no pipeline
# logic lives here.
#
#   experiments/coldrun.sh [N]      # N candidates per room (default 3)
#
# Needs FAL_API_KEY (restage) + ANTHROPIC_API_KEY (verify + select) in .env / env.
set -u
R="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$R/playbook/golden/.coldrun"; mkdir -p "$OUT"
P="$R/playbook/prompts/depth-t2i"
IMG="$R/playbook/golden/images"
N="${1:-3}"

BINDIR="$(mktemp -d)"; BIN="$BINDIR/director"
echo "building director…"
go build -C "$R" -o "$BIN" ./services/director || { echo "build failed"; exit 1; }

# id | room label | prompt file
ROOMS="zeniamar_living|living room|living
zeniamar_kitchen|kitchen|kitchen
zeniamar_bath|bathroom|bathroom
zeniamar_bedroom|bedroom|bedroom
rosas_living|living room|living
rosas_kitchen|kitchen|kitchen
rosas_bath|bathroom|bathroom
rosas_bath2|bathroom|bathroom
rosas_bedroom|bedroom|bedroom"

echo "cold run: best-of-$N, inspire bar, depth-t2i — 9 rooms across 2 properties"
while IFS='|' read -r id label pf; do
  [ -z "$id" ] && continue
  before="$IMG/${id}_before.jpg"
  [ -f "$before" ] || { echo "!! missing $before"; continue; }
  echo "=== $id ($label) ==="
  "$BIN" beststage --engine depth-t2i --mode inspire --n "$N" \
    --in "$before" --room "$label" --prompt "$(cat "$P/${pf}.txt")" \
    --out "$OUT/${id}_after.jpg" 2>&1 | sed 's/^/  /'
done <<< "$ROOMS"

echo "✓ cold run done → $OUT"
