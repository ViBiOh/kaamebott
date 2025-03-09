#!/usr/bin/env bash

set -o nounset -o pipefail -o errexit

main() {
  ./abitbol.js | jq >abitbol.json

  INDEXER_INPUT="abitbol.json" make run-indexer
}

main "${@}"
