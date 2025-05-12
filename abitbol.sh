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
  ./abitbol.js | jq >"$(script_dir)/pkg/indexer/indexes/abitbol.json"

  INDEXER_INPUT="abitbol.json" make run-indexer
}

main "${@}"
