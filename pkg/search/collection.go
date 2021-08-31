package search

import (
	"context"
	"database/sql"
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
	scanner := func(row *sql.Row) error {
		err := row.Scan(&id)
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}

	return id, a.dbApp.Get(ctx, scanner, getCollectionQuery, name)
}
