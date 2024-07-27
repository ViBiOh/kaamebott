#!/usr/bin/env bash

set -o nounset -o pipefail -o errexit

main() {
  curl --disable --silent --show-error --location --max-time 30 "https://kaamelott-soundboard.2ec0b4.fr/sounds/sounds.b2533e4b.json" | jq '[ .[] | with_entries(if .key == "title" then .key = "value" else . end) | .file |= rtrimstr(".mp3") | {id: .file, value: .value, character: .character, context: .episode, url: ("https://kaamelott-soundboard.2ec0b4.fr/#son/" + .file) } ]' >"kaamelott.json"

  curl --disable --silent --show-error --location --max-time 30 "https://raw.githubusercontent.com/kaamelott-gifboard/kaamelott-gifboard/main/gifs.json" | jq '[ .[] | {id: ("gif-" + .slug), value: .quote, character: (.characters_speaking|join(", ")), image: ("https://kaamelott-gifboard.fr/gifs/" + .filename) } ]' >"kaamelott_gif.json"

  INDEXER_INPUT="kaamelott.json" INDEXER_ENRICH="kaamelott_gif.json" make run-indexer
}

main "${@}"
