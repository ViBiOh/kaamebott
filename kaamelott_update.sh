#!/usr/bin/env bash

set -o nounset -o pipefail -o errexit

main() {
  local SCRIPT_DIR
  SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

  curl --disable --silent --show-error --location "https://raw.githubusercontent.com/ViBiOh/scripts/main/bootstrap.sh" | bash -s -- "-c" "var"
  source "${SCRIPT_DIR}/scripts/meta" && meta_check "var"

  var_read SOUND_VERSION

  curl --disable --silent --show-error --location --max-time 30 "https://kaamelott-soundboard.2ec0b4.fr/sounds/sounds.${SOUND_VERSION}.json" | jq '[ .[] | with_entries(if .key == "title" then .key = "value" else . end) | .file |= rtrimstr(".mp3") | {id: .file, value: .value, character: .character, context: .episode, url: ("https://kaamelott-soundboard.2ec0b4.fr/#son/" + .file) } ]' >"kaamelott.json"

  INDEXER_INPUT="kaamelott.json" make run-indexer
  rm "kaamelott.json"
}

main "${@}"
