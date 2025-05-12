#!/usr/bin/env bash

set -o nounset -o pipefail -o errexit

script_dir() {
  local FILE_SOURCE="${BASH_SOURCE[0]}"

  if [[ -L ${FILE_SOURCE} ]]; then
    dirname "$(readlink "${FILE_SOURCE}")"
  else
    (
      cd "$(dirname "${FILE_SOURCE}")" && pwd
    )
  fi
}

main() {
  local SCRIPT_DIR
  SCRIPT_DIR=$(script_dir)

  curl --disable --silent --show-error --location --max-time 30 "https://kaamelott-soundboard.2ec0b4.fr/sounds/sounds.b2533e4b.json" | jq '[ .[] | with_entries(if .key == "title" then .key = "value" else . end) | .file |= rtrimstr(".mp3") | {id: .file, value: .value, character: .character, context: .episode, url: ("https://kaamelott-soundboard.2ec0b4.fr/#son/" + .file) } ]' >"${SCRIPT_DIR}/pkg/indexer/indexes/kaamelott.json"

  curl --disable --silent --show-error --location --max-time 30 "https://raw.githubusercontent.com/kaamelott-gifboard/kaamelott-gifboard/main/gifs.json" | jq '[ .[] | {id: ("gif-" + .slug), value: .quote, character: (.characters_speaking|join(", ")), image: ("https://kaamelott-gifboard.fr/gifs/" + .filename) } ]' >"${SCRIPT_DIR}/pkg/indexer/indexes/kaamelott_next.json"

  INDEXER_INPUT="kaamelott.json" make run-indexer
}

main "${@}"
