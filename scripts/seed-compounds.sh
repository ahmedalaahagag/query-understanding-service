#!/usr/bin/env bash
# seed-compounds.sh — Seeds compound word rules (type=CMP) into the OpenSearch linguistic index.
#
# Reads TSV files from scripts/compound-data/{locale}.tsv with format:
#   compound<TAB>parts
#   icecream	ice cream
#   peanutbutter	peanut butter
#
# Each line produces a CMP entry in linguistic_{locale}.
# Lines starting with # are ignored.
#
# Usage:
#   ./scripts/seed-compounds.sh                          # Seed all locales
#   ./scripts/seed-compounds.sh http://localhost:9200     # Custom OS URL
#   ./scripts/seed-compounds.sh http://localhost:9200 en_gb  # Single locale
set -euo pipefail

OS_URL="${1:-http://localhost:9200}"
LOCALE_FILTER="${2:-}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DATA_DIR="${SCRIPT_DIR}/compound-data"

if [ ! -d "${DATA_DIR}" ]; then
  echo "Error: compound data directory not found: ${DATA_DIR}"
  echo "Create TSV files like: ${DATA_DIR}/en_gb.tsv"
  exit 1
fi

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

count=0

for file in "${DATA_DIR}"/*.tsv; do
  [ -f "$file" ] || continue

  locale=$(basename "$file" .tsv)

  # Skip if locale filter is set and doesn't match
  if [ -n "${LOCALE_FILTER}" ] && [ "${locale}" != "${LOCALE_FILTER}" ]; then
    continue
  fi

  index="linguistic_${locale}"
  ndjson_file="${TMPDIR}/${locale}_compounds.ndjson"
  > "${ndjson_file}"

  file_count=0
  while IFS=$'\t' read -r compound parts; do
    # Skip empty lines and comments
    [[ -z "$compound" || "$compound" == \#* ]] && continue

    echo '{"index":{}}' >> "${ndjson_file}"
    printf '{"term":"%s","variant":"%s","type":"CMP","locale":"%s"}\n' \
      "$compound" "$parts" "$locale" >> "${ndjson_file}"
    file_count=$((file_count + 1))
  done < "$file"

  if [ "$file_count" -eq 0 ]; then
    echo "  ${locale}: skipped (no entries)"
    continue
  fi

  # Bulk index
  result=$(curl -s -X POST "${OS_URL}/${index}/_bulk" \
    -H 'Content-Type: application/x-ndjson' \
    --data-binary "@${ndjson_file}")

  errors=$(echo "$result" | jq -r '.errors')
  indexed=$(echo "$result" | jq -r '.items | length')

  if [ "$errors" = "true" ]; then
    echo "  ${locale}: ${indexed} docs indexed (with errors)"
    echo "$result" | jq '.items[] | select(.index.error) | .index.error' 2>/dev/null
  else
    echo "  ${locale}: ${indexed} compound entries indexed"
  fi

  count=$((count + file_count))
done

echo "==> Done. ${count} total compound entries seeded."
