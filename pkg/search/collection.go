package search

import (
	"context"

	"github.com/jackc/pgx/v4"
)

const getCollectionQuery = `
SELECT
  id
FROM
  kaamebott.collection
WHERE
  name = $1
`

func (a App) getCollection(ctx context.Context, name string) (uint64, error) {
	var id uint64
	scanner := func(row pgx.Row) error {
		err := row.Scan(&id)
		if err == pgx.ErrNoRows {
			return nil
		}
		return err
	}

	return id, a.dbApp.Get(ctx, scanner, getCollectionQuery, name)
}
