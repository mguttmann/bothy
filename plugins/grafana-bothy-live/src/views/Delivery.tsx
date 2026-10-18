// SPDX-License-Identifier: Apache-2.0
/**
 * Delivery and sync (docs/spec/25-visualisierung.md 8.2, mirror of 6.3): delivery latency,
 * connecting, catch-up and fan-out lag as tiles with their latest value and a sparkline over the
 * dashboard period.
 */

import React from 'react';

import { lastValue, seriesOf } from '../frames';
import { formatValue } from '../format';
import { QUERIES } from '../queries.gen';
import { Sparkline } from './parts';
import type { ViewProps } from './types';

/**
 * The delivery and sync view: one tile per query of the view.
 *
 * @param props - frames and theme
 * @returns the rendering
 */
export function DeliveryView({ frames, theme }: ViewProps) {
  const c = theme.colors;
  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))', gap: 12 }}>
      {QUERIES.delivery.queries.map((q, i) => {
        const [s] = seriesOf(frames, q.refId);
        const value = s ? lastValue(s) : null;
        const series = c.viz.series[((i % 8) + 1) as 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8];
        return (
          <div
            key={q.refId}
            role="group"
            aria-label={q.legend}
            style={{
              background: c.surface.raised,
              border: `1px solid ${c.border.subtle}`,
              borderRadius: theme.radii.md,
              padding: 12,
            }}
          >
            <div style={{ color: c.text.secondary, fontSize: theme.sizes[12] }}>{q.legend}</div>
            <div style={{ fontFamily: theme.fontMono, fontSize: theme.sizes[24], margin: '4px 0' }}>
              {formatValue(value, q.unit)}
            </div>
            {s && <Sparkline values={s.values} color={series} testId={`spark-${q.refId}`} />}
          </div>
        );
      })}
    </div>
  );
}
