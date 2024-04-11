#!/usr/bin/env bash

set -o nounset -o pipefail -o errexit

main() {
  curl --disable --silent --show-error --location --max-time 30 "https://raw.githubusercontent.com/kaamelott-gifboard/kaamelott-gifboard/main/gifs.json" | jq '[ .[] | {id: ("gif-" + .slug), value: .quote, character: (.characters_speaking|join(", ")), image: ("https://kaamelott-gifboard.fr/gifs/" + .filename) } ]' >"kaamelott.json"

  INDEXER_INPUT="kaamelott.json" INDEXER_ENRICH="true" make run-indexer
  rm "kaamelott.json"
}

main "${@}"
