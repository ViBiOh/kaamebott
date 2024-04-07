package search

import (
	"context"
	"fmt"
	"strings"

	"github.com/ViBiOh/kaamebott/pkg/model"
	"github.com/jackc/pgx/v5"
)

const searchQuoteQuery = `
SELECT
  q.id,
  q.value,
  q.character,
  q.context,
  q.url,
  q.image,,
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

func computeQuoteQuery(collectionID uint64, language, last string, words []string) (string, []any) {
	query := strings.Builder{}
	query.WriteString(searchQuoteQuery)

	args := []any{
		collectionID,
	}

	if len(words) != 0 {
		args = append(args, strings.Join(words, " & "))
		query.WriteString(fmt.Sprintf(" AND q.search_vector @@ to_tsquery('%s', $%d)", language, len(args)))
	}

	if len(last) != 0 {
		args = append(args, last)
		query.WriteString(fmt.Sprintf(" AND q.id > $%d", len(args)))
	}

	query.WriteString(searchQuoteTail)

	return query.String(), args
}

func (s Service) searchQuote(ctx context.Context, collectionID uint64, language, query, last string) (model.Quote, error) {
	var totalCount uint
	var item model.Quote

	scanner := func(row pgx.Row) error {
		err := row.Scan(&item.ID, &item.Value, &item.Character, &item.Context, &item.URL, &item.Image, &item.Collection, &totalCount)

		if err == pgx.ErrNoRows {
			return nil
		}
		return err
	}

	words, err := getWords(query)
	if err != nil {
		return item, fmt.Errorf("get words: %w", err)
	}

	sqlQuery, sqlArgs := computeQuoteQuery(collectionID, language, last, words)
	item.Language = language

	return item, s.db.Get(ctx, scanner, sqlQuery, sqlArgs...)
}

const getQuoteQuery = `
SELECT
  q.id,
  q.value,
  q.character,
  q.context,
  q.url,
  q.image,
  c.name
FROM
  kaamebott.quote q
INNER JOIN
  kaamebott.collection c ON c.id = q.collection_id
WHERE
  q.collection_id = $1
  AND q.id = $2
`

func (s Service) getQuote(ctx context.Context, collectionID uint64, language, id string) (model.Quote, error) {
	var item model.Quote

	scanner := func(row pgx.Row) error {
		err := row.Scan(&item.ID, &item.Value, &item.Character, &item.Context, &item.URL, &item.Image, &item.Collection)

		if err == pgx.ErrNoRows {
			return nil
		}
		return err
	}

	item.Language = language

	return item, s.db.Get(ctx, scanner, getQuoteQuery, collectionID, id)
}

const getRandomQuoteQuery = `
SELECT
  q.id,
  q.value,
  q.character,
  q.context,
  q.url,
  q.image,
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

func (a Service) getRandomQuote(ctx context.Context, collectionID uint64, language string) (model.Quote, error) {
	var item model.Quote

	scanner := func(row pgx.Row) error {
		err := row.Scan(&item.ID, &item.Value, &item.Character, &item.Context, &item.URL, &item.Image, &item.Collection)
		if err == pgx.ErrNoRows {
			return nil
		}
		return err
	}

	item.Language = language

	return item, a.db.Get(ctx, scanner, getRandomQuoteQuery, collectionID)
}
