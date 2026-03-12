#!/usr/bin/env bash
#
# Copies OpenSearch indexes from a source cluster to a destination cluster.
# Uses scroll + bulk API since the clusters are in separate VPCs.
#
# Usage:
#   ./scripts/copy-indexes.sh                  # copy all 3 indexes
#   ./scripts/copy-indexes.sh hellofresh_gb_products_a  # copy one index
#
set -euo pipefail

# --- Source cluster (MXP staging) ---
SRC_URL="https://vpc-hf-mxp-v2-stg-tf-4ijugsdo35gysgs4wyxpee4nk4.eu-west-1.es.amazonaws.com"
SRC_USER="master"
SRC_PASS='WDdEAe+7FE1rg1Ral2V9K2p['

# --- Destination cluster (QUS test search) ---
DST_URL="https://vpc-test-search-dlphznvzrujlwfzg4ex3jeqkum.eu-west-1.es.amazonaws.com"
DST_USER="master"
DST_PASS='V&77F)"V1jVz'

# --- Indexes to copy ---
ALL_INDEXES=(
  hellofresh_gb_products_a
  hellofresh_us_products_a
  hellofresh_ca_products_a
)

SCROLL_SIZE=500
SCROLL_TTL="5m"
BULK_SIZE=500

# Use provided index or all
if [[ $# -gt 0 ]]; then
  INDEXES=("$@")
else
  INDEXES=("${ALL_INDEXES[@]}")
fi

src_curl() {
  curl -s -u "${SRC_USER}:${SRC_PASS}" -H "Content-Type: application/json" "$@"
}

dst_curl() {
  curl -s -u "${DST_USER}:${DST_PASS}" -H "Content-Type: application/json" "$@"
}

copy_index() {
  local index="$1"
  echo ""
  echo "=== Copying index: ${index} ==="

  # 1. Get source doc count
  local src_count
  src_count=$(src_curl "${SRC_URL}/${index}/_count" | jq -r '.count')
  echo "Source doc count: ${src_count}"

  if [[ "${src_count}" == "0" || "${src_count}" == "null" ]]; then
    echo "SKIP: index ${index} is empty or does not exist on source"
    return
  fi

  # 2. Get mapping + settings from source
  echo "Fetching mapping and settings..."
  local index_config
  index_config=$(src_curl "${SRC_URL}/${index}")

  local mapping settings
  mapping=$(echo "${index_config}" | jq -r ".\"${index}\".mappings")
  settings=$(echo "${index_config}" | jq -r ".\"${index}\".settings.index")

  # Keep only transferable settings; force replicas to 2 for 3-AZ dest cluster.
  local clean_settings
  clean_settings=$(echo "${settings}" | jq 'del(.[] | nulls)' | jq '{
    number_of_shards,
    number_of_replicas: "2",
    analysis,
    similarity,
    knn
  } | del(.[] | nulls)')

  # 3. Check if destination index exists
  local dst_exists
  dst_exists=$(curl -s -o /dev/null -w "%{http_code}" -u "${DST_USER}:${DST_PASS}" "${DST_URL}/${index}")

  if [[ "${dst_exists}" == "200" ]]; then
    local dst_count
    dst_count=$(dst_curl "${DST_URL}/${index}/_count" | jq -r '.count')
    echo "Destination index already exists with ${dst_count} docs"
    read -rp "Delete and recreate? [y/N] " confirm
    if [[ "${confirm}" != "y" && "${confirm}" != "Y" ]]; then
      echo "SKIP: ${index}"
      return
    fi
    echo "Deleting destination index..."
    dst_curl -X DELETE "${DST_URL}/${index}" > /dev/null
  fi

  # 4. Create destination index with mapping + settings
  echo "Creating index on destination..."
  local create_body
  create_body=$(jq -n \
    --argjson settings "${clean_settings}" \
    --argjson mappings "${mapping}" \
    '{ settings: $settings, mappings: $mappings }')

  dst_curl -X PUT "${DST_URL}/${index}" -d "${create_body}" > /dev/null
  echo "Index created"

  # 5. Scroll through source and bulk insert into destination
  echo "Copying documents (scroll_size=${SCROLL_SIZE})..."

  local tmpdir
  tmpdir=$(mktemp -d)
  trap "rm -rf ${tmpdir}" RETURN

  local scroll_response scroll_id hits total_copied=0

  src_curl "${SRC_URL}/${index}/_search?scroll=${SCROLL_TTL}" \
    -d "{\"size\": ${SCROLL_SIZE}, \"query\": {\"match_all\": {}}}" \
    > "${tmpdir}/scroll.json"

  scroll_id=$(jq -r '._scroll_id' < "${tmpdir}/scroll.json")
  hits=$(jq -r '.hits.hits | length' < "${tmpdir}/scroll.json")

  while [[ "${hits}" -gt 0 ]]; do
    # Build bulk ndjson payload from scroll hits
    jq -c '.hits.hits[] | {"index":{"_index":"'"${index}"'","_id":._id}}, ._source' \
      < "${tmpdir}/scroll.json" > "${tmpdir}/bulk.ndjson"
    # Bulk requires a trailing newline
    echo "" >> "${tmpdir}/bulk.ndjson"

    # Send bulk request (must not use dst_curl – it sets Content-Type: application/json
    # which conflicts with the required application/x-ndjson for bulk).
    local bulk_result
    bulk_result=$(curl -s -u "${DST_USER}:${DST_PASS}" \
      -X POST "${DST_URL}/_bulk" \
      -H "Content-Type: application/x-ndjson" \
      --data-binary "@${tmpdir}/bulk.ndjson")

    # Check for errors
    local has_errors
    has_errors=$(echo "${bulk_result}" | jq -r '.errors // false')
    if [[ "${has_errors}" == "true" ]]; then
      local err_count
      err_count=$(echo "${bulk_result}" | jq '[.items[] | select(.index.error)] | length')
      echo ""
      echo "  WARNING: ${err_count} bulk errors in this batch"
    fi

    total_copied=$((total_copied + hits))
    printf "\r  Copied %d / %d docs" "${total_copied}" "${src_count}"

    # Next scroll page
    src_curl "${SRC_URL}/_search/scroll" \
      -d "{\"scroll\": \"${SCROLL_TTL}\", \"scroll_id\": \"${scroll_id}\"}" \
      > "${tmpdir}/scroll.json"

    scroll_id=$(jq -r '._scroll_id' < "${tmpdir}/scroll.json")
    hits=$(jq -r '.hits.hits | length' < "${tmpdir}/scroll.json")
  done

  echo ""

  # Clear scroll
  src_curl -X DELETE "${SRC_URL}/_search/scroll" \
    -d "{\"scroll_id\": \"${scroll_id}\"}" > /dev/null 2>&1 || true

  rm -rf "${tmpdir}"
  trap - RETURN

  # 6. Refresh and verify
  echo "Refreshing index..."
  dst_curl -X POST "${DST_URL}/${index}/_refresh" > /dev/null
  local dst_final
  dst_final=$(dst_curl "${DST_URL}/${index}/_count" | jq -r '.count')
  echo "Done: ${dst_final} / ${src_count} docs copied to ${index}"

  if [[ "${dst_final}" != "${src_count}" ]]; then
    echo "WARNING: doc count mismatch (source=${src_count}, dest=${dst_final})"
  fi
}

# --- Main ---
echo "Source:      ${SRC_URL}"
echo "Destination: ${DST_URL}"
echo "Indexes:     ${INDEXES[*]}"
echo ""

# Check connectivity
echo "Checking source cluster..."
src_curl "${SRC_URL}/_cluster/health" | jq -r '"  status: \(.status), nodes: \(.number_of_nodes)"'

echo "Checking destination cluster..."
dst_curl "${DST_URL}/_cluster/health" | jq -r '"  status: \(.status), nodes: \(.number_of_nodes)"'

for index in "${INDEXES[@]}"; do
  copy_index "${index}"
done

echo ""
echo "All done."
