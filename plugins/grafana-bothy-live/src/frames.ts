// SPDX-License-Identifier: Apache-2.0
/**
 * Reading Grafana data frames: the views get series by the refId of their queries
 * (src/queries.gen.ts) and never look at a frame of another refId.
 */

import { type DataFrame, FieldType } from '@grafana/data';

/** One series: the labels of the value field with its points in time order. */
export interface Series {
  /** Prometheus labels of the series (agent handle, state, SLO name, ...); never an ID (17 rule 9). */
  readonly labels: Readonly<Record<string, string>>;
  /** Times in epoch milliseconds, ascending. */
  readonly times: readonly number[];
  /** Values at `times`; null or NaN where Prometheus had no value. */
  readonly values: ReadonlyArray<number | null>;
}

function finite(v: unknown): number | null {
  return typeof v === 'number' && Number.isFinite(v) ? v : null;
}

/**
 * The series of every frame whose refId is refId: one series per number field.
 *
 * @param frames - the data frames of the panel
 * @param refId - refId of a query of the view
 * @returns the series; empty when no frame has the refId
 */
export function seriesOf(frames: readonly DataFrame[], refId: string): Series[] {
  const out: Series[] = [];
  for (const frame of frames) {
    if (frame.refId !== refId) {
      continue;
    }
    const time = frame.fields.find((f) => f.type === FieldType.time);
    const times = time ? Array.from(time.values as ArrayLike<unknown>, (t) => Number(t)) : [];
    for (const field of frame.fields) {
      if (field.type !== FieldType.number) {
        continue;
      }
      const values = Array.from(field.values as ArrayLike<unknown>, finite);
      out.push({ labels: { ...(field.labels ?? {}) }, times: times.length ? times : values.map((_, i) => i), values });
    }
  }
  return out;
}

/**
 * The last value of a series that is a number.
 *
 * @param s - a series
 * @returns the value, or null when the series has none
 */
export function lastValue(s: Series): number | null {
  for (let i = s.values.length - 1; i >= 0; i--) {
    const v = s.values[i];
    if (v !== null && v !== undefined) {
      return v;
    }
  }
  return null;
}

/**
 * The value a series had at time t: its last point at or before t.
 *
 * @param s - a series
 * @param t - epoch milliseconds
 * @returns the value, or null before the first point or where that point has no value
 */
export function valueAt(s: Series, t: number): number | null {
  let found: number | null = null;
  for (let i = 0; i < s.times.length; i++) {
    const time = s.times[i] ?? Number.POSITIVE_INFINITY;
    if (time > t) {
      break;
    }
    found = s.values[i] ?? null;
  }
  return found;
}

/**
 * A label of a series.
 *
 * @param s - a series
 * @param key - the Prometheus label name
 * @returns its value, or an empty string when the series lacks it
 */
export function label(s: Series, key: string): string {
  return s.labels[key] ?? '';
}

/**
 * Sum of the last values of the series, skipping series without a value.
 *
 * @param series - the series to add
 * @returns the sum, or null when no series has a value
 */
export function sumLast(series: readonly Series[]): number | null {
  let sum: number | null = null;
  for (const s of series) {
    const v = lastValue(s);
    if (v !== null) {
      sum = (sum ?? 0) + v;
    }
  }
  return sum;
}
