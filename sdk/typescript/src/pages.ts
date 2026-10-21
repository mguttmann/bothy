// SPDX-License-Identifier: Apache-2.0
/**
 * Iterators over cursor-based lists (docs/spec/06-api.md 14 point 1): request with `cursor`,
 * answer `{ items, next_cursor }`; a page without `next_cursor` is the last one. Browser-safe.
 *
 * @example
 * ```ts
 * for await (const user of pageItems((cursor) =>
 *   unwrap(api.GET("/users", { params: { query: { cursor } } }), "listUsers"),
 * )) console.log(user.display_name);
 * ```
 *
 * @module
 */

/** One page of a cursor-based list. */
export interface CursorPage<C = string> {
  /** The cursor of the next page; absent (or `null`) on the last page. */
  readonly next_cursor?: C | null | undefined;
}

/** Fetches the page at `cursor` (`undefined` for the first page). */
export type PageFetcher<P, C = string> = (cursor: C | undefined) => Promise<P>;

/**
 * Yields every page of a list, following `next_cursor` until a page has none.
 *
 * A failed page rejects the iteration; breaking out of the loop fetches no further page.
 *
 * @param fetchPage - Fetches one page.
 * @param options - `cursor` to start at a known cursor instead of the first page.
 * @returns An async iterator over the pages.
 */
export async function* pages<P extends CursorPage<C>, C = string>(
  fetchPage: PageFetcher<P, C>,
  options: { cursor?: C | undefined } = {},
): AsyncGenerator<P, void, undefined> {
  let cursor = options.cursor;
  for (;;) {
    const page = await fetchPage(cursor);
    yield page;
    const next = page.next_cursor;
    if (next === undefined || next === null) return;
    cursor = next;
  }
}

/**
 * Yields every item of every page of a list.
 *
 * @param fetchPage - Fetches one page with an `items` array.
 * @param options - `cursor` to start at a known cursor.
 * @returns An async iterator over the items.
 */
export async function* pageItems<T, C = string>(
  fetchPage: PageFetcher<CursorPage<C> & { readonly items: readonly T[] }, C>,
  options: { cursor?: C | undefined } = {},
): AsyncGenerator<T, void, undefined> {
  for await (const page of pages(fetchPage, options)) yield* page.items;
}
