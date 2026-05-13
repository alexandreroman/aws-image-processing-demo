#!/usr/bin/env bash
# Download 50 sample images from Picsum into
# frontend/public/sample-images/ as 1.jpg .. 50.jpg.
#
# License: Picsum (https://picsum.photos) serves CC0
# photographs by Unsplash photographers, free for any
# use, no attribution required.
#
# A handful of output indexes (7, 18, 29, 38, 47) are
# fetched with `?blur=4` so the pipeline gets exercised
# against intentionally degraded inputs — the
# LLM-based describe activity should still produce
# reasonable output.

set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
samples_dir="${repo_root}/frontend/public/sample-images"

mkdir -p "${samples_dir}"

# Curated Picsum IDs, in the order they map to output
# indexes 1..50. Themes span nature, urban, landscape,
# people, objects, and animals.
picsum_ids=(
  1   10  13  15  17  20  27  28  29  30
  36  37  42  48  60  76  80  87  96  100
  110 116 119 122 129 134 145 152 164 175
  180 191 200 210 225 237 244 257 269 274
  281 290 308 325 339 344 356 367 378 384
)

# Output indexes that should be intentionally blurred.
blurred_indexes=(7 18 29 38 47)
is_blurred() {
  local needle="$1"
  local idx
  for idx in "${blurred_indexes[@]}"; do
    if [[ "${idx}" == "${needle}" ]]; then
      return 0
    fi
  done
  return 1
}

total="${#picsum_ids[@]}"

for (( i = 0; i < total; i++ )); do
  out_index=$(( i + 1 ))
  picsum_id="${picsum_ids[i]}"
  url="https://picsum.photos/id/${picsum_id}/1600/1200.jpg"
  tag=""
  if is_blurred "${out_index}"; then
    url="${url}?blur=4"
    tag=" (blurred)"
  fi
  out_file="${samples_dir}/${out_index}.jpg"

  printf '[%2d/%d] id=%s%s -> %d.jpg\n' \
    "${out_index}" "${total}" "${picsum_id}" "${tag}" "${out_index}"

  curl -fsSL --retry 3 --retry-delay 1 --max-time 30 -o "${out_file}" "${url}"
done

echo
echo "Done: ${total} image(s) saved to ${samples_dir}/"
