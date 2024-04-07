--- clean
DROP TABLE IF EXISTS kaamebott.quote;
DROP TABLE IF EXISTS kaamebott.collection;

DROP INDEX IF EXISTS quote_search;
DROP INDEX IF EXISTS quote_collection_id;
DROP INDEX IF EXISTS quote_key;
DROP INDEX IF EXISTS collection_id;
DROP INDEX IF EXISTS collection_name;

DROP SEQUENCE IF EXISTS kaamebott.collection_seq;

DROP SCHEMA IF EXISTS kaamebott;

-- extension
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS unaccent;

-- schema
CREATE SCHEMA kaamebott;

-- collection
CREATE SEQUENCE kaamebott.collection_seq;
CREATE TABLE kaamebott.collection (
  id BIGINT NOT NULL DEFAULT nextval('kaamebott.collection_seq'),
  name TEXT NOT NULL,
  language TEXT NOT NULL DEFAULT 'french'
);
ALTER SEQUENCE kaamebott.collection_seq OWNED BY kaamebott.collection.id;

CREATE UNIQUE INDEX collection_id ON kaamebott.collection(id);
CREATE UNIQUE INDEX collection_name ON kaamebott.collection(name);

-- quote
CREATE TABLE kaamebott.quote (
  collection_id BIGINT NOT NULL REFERENCES kaamebott.collection(id) ON DELETE CASCADE,
  id TEXT NOT NULL,
  value TEXT NOT NULL,
  character TEXT NOT NULL,
  context TEXT NOT NULL,
  url TEXT NOT NULL,
  image TEXT,
  search_vector TSVECTOR
);

CREATE UNIQUE INDEX quote_key ON kaamebott.quote(collection_id, id);
CREATE INDEX quote_collection_id ON kaamebott.quote(collection_id);
CREATE INDEX quote_search ON kaamebott.quote USING gin(search_vector);
