package search

import (
	"context"
	"fmt"
	"strings"

	"github.com/ViBiOh/kaamebott/pkg/model"
	"github.com/jackc/pgx/v4"
)

const searchQuoteQuery = `
SELECT
  q.id,
  q.value,
  q.character,
  q.context,
  c.name,
  count(1) OVER() AS full_count
FROM
  kaamebott.quote q
INNER JOIN
  kaamebott.collection c ON c.id = q.collection_id
WHERE
  q.collection_id = $1
`

const searchQuoteTail = `
ORDER BY
  q.id ASC
LIMIT 1
`

func computeQuoteQuery(collectionID uint64, last string, words []string) (string, []interface{}) {
	query := strings.Builder{}
	query.WriteString(searchQuoteQuery)

	args := []interface{}{
		collectionID,
	}

	if len(words) != 0 {
		args = append(args, strings.Join(words, " & "))
		query.WriteString(fmt.Sprintf(" AND q.search_vector @@ to_tsquery('french', $%d)", len(args)))
	}

	if len(last) != 0 {
		args = append(args, last)
		query.WriteString(fmt.Sprintf(" AND q.id > $%d", len(args)))
	}

	query.WriteString(searchQuoteTail)

	return query.String(), args
}

func (a App) searchQuote(ctx context.Context, collectionID uint64, query, last string) (model.Quote, error) {
	var words []string
	if len(query) > 0 {
		words = strings.Split(query, " ")
	}

	var totalCount uint
	var item model.Quote

	scanner := func(row pgx.Row) error {
		err := row.Scan(&item.ID, &item.Value, &item.Character, &item.Context, &item.Collection, &totalCount)

		if err == pgx.ErrNoRows {
			return nil
		}
		return err
	}

	sqlQuery, sqlArgs := computeQuoteQuery(collectionID, last, words)

	return item, a.dbApp.Get(ctx, scanner, sqlQuery, sqlArgs...)
}

const getQuoteQuery = `
SELECT
  q.id,
  q.value,
  q.character,
  q.context,
  c.name
FROM
  kaamebott.quote q
INNER JOIN
  kaamebott.collection c ON c.id = q.collection_id
WHERE
  q.collection_id = $1
  AND q.id = $2
`

func (a App) getQuote(ctx context.Context, collectionID uint64, id string) (model.Quote, error) {
	var item model.Quote

	scanner := func(row pgx.Row) error {
		err := row.Scan(&item.ID, &item.Value, &item.Character, &item.Context, &item.Collection)

		if err == pgx.ErrNoRows {
			return nil
		}
		return err
	}

	return item, a.dbApp.Get(ctx, scanner, getQuoteQuery, collectionID, id)
}

const getRandomQuoteQuery = `
SELECT
  q.id,
  q.value,
  q.character,
  q.context,
  c.name
FROM
  kaamebott.quote q
INNER JOIN
  kaamebott.collection c ON c.id = q.collection_id
WHERE
  q.collection_id = $1
OFFSET floor(random() * (
    SELECT
      COUNT(1)
    FROM
      kaamebott.quote
    WHERE
      collection_id = $1
  )
)
LIMIT 1
`

func (a App) getRandomQuote(ctx context.Context, collectionID uint64) (model.Quote, error) {
	var item model.Quote

	scanner := func(row pgx.Row) error {
		err := row.Scan(&item.ID, &item.Value, &item.Character, &item.Context, &item.Collection)
		if err == pgx.ErrNoRows {
			return nil
		}
		return err
	}

	return item, a.dbApp.Get(ctx, scanner, getRandomQuoteQuery, collectionID)
}
