#!/usr/bin/env bash

set -o nounset -o pipefail -o errexit

main() {
  # Coming from https://github.com/trazip/oss-117-api/blob/34ea47e2dc7e81453cf5866373d433ff8e69378d/db/seeds.rb
  # Coming from https://github.com/shevabam/oss-117-quotes-api/blob/main/datas.json

  INDEXER_INPUT="oss117.json" make run-indexer
}

main "${@}"
