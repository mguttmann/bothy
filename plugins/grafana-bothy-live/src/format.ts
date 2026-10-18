// SPDX-License-Identifier: Apache-2.0
/** Text of a value in the unit its query declares (queries.yaml field `unit`). */

import type { QueryUnit } from './queries.gen';

/** Text for a missing value. */
export const NO_DATA = 'no data';

/**
 * Formats a value for display.
 *
 * @param value - the number, or null for no data
 * @param unit - the unit of the query
 * @returns the text, for example `3`, `12.5/min`, `120 ms`, `1.25 s`, `99.95 %` or `0.42`
 */
export function formatValue(value: number | null, unit: QueryUnit): string {
  if (value === null || !Number.isFinite(value)) {
    return NO_DATA;
  }
  switch (unit) {
    case 'count':
      return String(Math.round(value));
    case 'per_minute':
      return `${value.toFixed(1)}/min`;
    case 'seconds':
      return value < 1 ? `${Math.round(value * 1000)} ms` : `${value.toFixed(2)} s`;
    case 'ratio':
      return `${(value * 100).toFixed(2)} %`;
    case 'currency':
      return value.toFixed(2);
  }
}
