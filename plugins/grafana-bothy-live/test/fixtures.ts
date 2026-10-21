// SPDX-License-Identifier: Apache-2.0
/**
 * Fixed data frames for the view tests, shaped like the frames Grafana's Prometheus data source
 * returns: one frame per series, a time field and a number field with the series labels.
 * Agent handles are white-label sample names (Scout, Lotse).
 */

import { createDataFrame, type DataFrame, FieldType } from '@grafana/data';

/** Start of the fixed period: 2026-10-10T12:00:00Z. */
export const T0 = Date.UTC(2026, 9, 10, 12, 0, 0);
/** One minute in milliseconds. */
export const MINUTE = 60_000;

/**
 * One series frame.
 *
 * @param refId - refId of the query
 * @param labels - series labels
 * @param values - values, one per minute from T0 (or at times)
 * @param times - optional explicit times
 * @returns the frame
 */
export function frame(
  refId: string,
  labels: Record<string, string>,
  values: Array<number | null>,
  times?: number[]
): DataFrame {
  return createDataFrame({
    refId,
    fields: [
      { name: 'Time', type: FieldType.time, values: times ?? values.map((_, i) => T0 + i * MINUTE) },
      { name: 'Value', type: FieldType.number, values, labels },
    ],
  });
}
