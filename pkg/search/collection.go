package search

import (
	"context"

	"github.com/jackc/pgx/v5"
)

const getCollectionQuery = `
SELECT
  id,
  language
FROM
  kaamebott.collection
WHERE
  name = $1
`

func (a App) getCollection(ctx context.Context, name string) (id uint64, language string, err error) {
	scanner := func(row pgx.Row) error {
		scanErr := row.Scan(&id, &language)
		if scanErr == pgx.ErrNoRows {
			return nil
		}
		return scanErr
	}

	err = a.dbApp.Get(ctx, scanner, getCollectionQuery, name)
	return
}
