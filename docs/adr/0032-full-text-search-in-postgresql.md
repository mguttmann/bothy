---
status: accepted
date: 2026-10-10
decision-makers: Planning lead, Team G platform services
---

# Full-text search in PostgreSQL

## Context and Problem Statement

People and agents search in messages, thread titles, file names, transcripts and document texts.
Every hit must be filtered by permissions (B7, P2); counts and snippets must not reveal anything
from channels the searcher cannot access. The data lives in PostgreSQL (B3). Where and how is
search done?

## Decision Drivers

- Permission filter in the query, from one source of truth (`authz`).
- No second data store that must be kept in sync, backed up and deleted.
- Deletion (also under GDPR) takes effect on search immediately.
- German and English with word stems; identifiers and names exact.
- Operation on a single machine; later scaling possible.

## Considered Options

- PostgreSQL full text (`tsvector`, GIN, `websearch_to_tsquery`) behind a port
- External search engine (for example Meilisearch, OpenSearch)
- Trigram search (`pg_trgm`) or `ILIKE`
- Only search in the client

## Decision Outcome

Chosen option: "PostgreSQL full text behind a port", because it needs no further service, stays
consistent in the same transaction and carries the permission filter as a simple condition in
SQL. Generated columns with language stems and unchanged words, a query as OR over the instance
languages, a candidate limit before ranking, highlighting with `ts_headline`. Details:
[14-search.md](../spec/14-search.md).

### Consequences

- Good, because search vectors always match the content without triggers or jobs; a deleted
  message immediately has an empty vector.
- Good, because backup, restore and migration take search along without a rebuild.
- Good, because `websearch_to_tsquery` accepts raw input without syntax errors [Q1, verified].
- Bad, because German compounds are not split [Q2, verified]; "Rechnung" does not find
  "Rechnungsadresse".
- Bad, because there is no tolerance for typos.
- Bad, because ranking is expensive for many hits [Q1, verified]; a candidate limit bounds the
  costs but makes very old hits for frequent words reachable only through filters.
- Bad, because a change of the vector expressions rewrites the table and needs a
  maintenance window.

### Confirmation

- Language tests (German, English, identifiers) and a test that pins the compound limitation.
- Permission tests: the same query by two principals with different grants; no hits from
  channels and DMs they cannot access.
- Load test in the release workflow: p95 below `search.p95_target` at `search.benchmark_messages`
  and the prefix search below `search.prefix_p95_target` (`services/server/perf/search`, in
  `make perf` and by hand in `make bench-search`, M3-16).
- Migration lint: generated search columns only with `STORED`.

## Pros and Cons of the Options

### PostgreSQL full text behind a port

- Good, because one data path, one source of truth, transactional.
- Good, because the port allows an external adapter later.
- Neutral, because languages come through built-in configurations (`german`, `english`,
  `simple` and others).
- Bad, because compounds and typos are not covered.

### External search engine

- Good, because typo tolerance, facets and better relevance are built in.
- Bad, because another service must be operated, backed up and monitored.
- Bad, because an index outside the database must be kept in sync; deletions
  and permission changes arrive with a delay.
- Bad, because it creates the temptation to map permissions in the index (a second permission
  logic).

### Trigram search or `ILIKE`

- Good, because partial words and similarity are found.
- Bad, because there are no word stems and no ranking by relevance, and trigram indexes on
  long texts become large.
- Remains possible as an intermediate step for titles and file names.

### Only search in the client

- Good, because there is no server effort.
- Bad, because only cached messages are found; older content is missing.
- Remains as a supplement for the offline case.

## More Information

- Sources (retrieved 2026-10-10): Q1 https://www.postgresql.org/docs/18/textsearch-controls.html
  [verified]; Q2 https://www.postgresql.org/docs/current/textsearch-dictionaries.html [verified].
- The queries from 14-search were run on a throwaway instance of PostgreSQL 16.
- Criteria for an external search engine: [14-search.md](../spec/14-search.md) section 9.
