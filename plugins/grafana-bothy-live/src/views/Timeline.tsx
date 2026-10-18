// SPDX-License-Identifier: Apache-2.0
/**
 * Timeline (docs/spec/25-visualisierung.md 8.2, mirror of 6.5): plays the dashboard period back.
 * The area shows the active runs over the period, the playhead a vertical line; the table below
 * lists every query of the view at the playhead. Controls: play, pause, jump to start or end and
 * the speeds of `viz.timeline_speeds` (seconds of time of day per second).
 */

import { dateTimeFormat } from '@grafana/data';
import React, { useEffect, useState } from 'react';

import { seriesOf, valueAt, type Series } from '../frames';
import { formatValue } from '../format';
import { QUERIES } from '../queries.gen';
import { tableStyles } from './parts';
import type { ViewProps } from './types';

/** Playback speeds in seconds of time of day per second, as `viz.timeline_speeds` (25 section 15). */
export const SPEEDS = [60, 300, 900] as const;
/** Milliseconds between two playhead steps. */
export const TICK_MS = 250;

/**
 * The playhead after one tick.
 *
 * @param playhead - current playhead in epoch milliseconds
 * @param speed - seconds of time of day per second
 * @param to - end of the period
 * @returns the new playhead, never after to
 */
export function advance(playhead: number, speed: number, to: number): number {
  return Math.min(to, playhead + speed * TICK_MS);
}

/** The total of several series at each time of the first one (active runs over all states). */
function totals(series: readonly Series[]): Array<[number, number]> {
  const times = [...new Set(series.flatMap((s) => s.times))].sort((a, b) => a - b);
  return times.map((t) => [t, series.reduce((sum, s) => sum + (valueAt(s, t) ?? 0), 0)]);
}

const WIDTH = 600;
const HEIGHT = 80;

/**
 * The timeline view.
 *
 * @param props - frames, theme, the dashboard period and time zone
 * @returns the rendering
 */
export function TimelineView({ frames, theme, range, timeZone }: ViewProps) {
  const [state, setPlayhead] = useState(range.from);
  // A dashboard refresh moves a relative period; the playhead stays where it was, kept inside it.
  const playhead = Math.min(Math.max(state, range.from), range.to);
  const [playing, setPlaying] = useState(false);
  const [speed, setSpeed] = useState<number>(SPEEDS[0]);
  const st = tableStyles(theme);
  const c = theme.colors;

  useEffect(() => {
    if (!playing) {
      return undefined;
    }
    const timer = setInterval(() => {
      setPlayhead((p) => {
        const next = advance(Math.max(p, range.from), speed, range.to);
        if (next >= range.to) {
          setPlaying(false);
        }
        return next;
      });
    }, TICK_MS);
    return () => clearInterval(timer);
  }, [playing, speed, range.from, range.to]);

  const queries = QUERIES.timeline.queries;
  const bySeries = queries.map((q) => ({ q, series: seriesOf(frames, q.refId) }));
  if (bySeries.every(({ series }) => series.length === 0)) {
    return <p style={st.empty}>No data in this period.</p>;
  }

  const span = Math.max(1, range.to - range.from);
  const x = (t: number) => ((t - range.from) / span) * WIDTH;
  const active = totals(bySeries[0]?.series ?? []);
  const max = Math.max(1, ...active.map(([, v]) => v));
  const y = (v: number) => HEIGHT - (v / max) * HEIGHT;
  const line = active.map(([t, v]) => `${x(t).toFixed(1)},${y(v).toFixed(1)}`).join(' ');
  const first = active[0];
  const last = active[active.length - 1];
  const area = first && last ? `${x(first[0]).toFixed(1)},${HEIGHT} ${line} ${x(last[0]).toFixed(1)},${HEIGHT}` : '';
  const button = {
    background: c.surface.raised,
    color: c.text.primary,
    border: `1px solid ${c.border.control}`,
    borderRadius: theme.radii.sm,
    padding: '2px 8px',
    marginRight: 6,
    font: 'inherit',
    cursor: 'pointer',
  } as React.CSSProperties;

  const rows = bySeries.flatMap(({ q, series }) =>
    series.length > 1 || (series[0] && Object.keys(series[0].labels).length > 0)
      ? series.map((s) => ({
          key: `${q.refId}-${JSON.stringify(s.labels)}`,
          name: Object.values(s.labels).join(' ') || q.legend,
          text: formatValue(valueAt(s, playhead), q.unit),
        }))
      : [{ key: q.refId, name: q.legend, text: formatValue(series[0] ? valueAt(series[0], playhead) : null, q.unit) }]
  );

  return (
    <div>
      <div style={{ marginBottom: 8, display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 4 }}>
        <button type="button" style={button} onClick={() => setPlayhead(range.from)}>
          Jump to start
        </button>
        <button type="button" style={button} onClick={() => setPlaying((p) => !p && playhead < range.to)}>
          {playing ? 'Pause' : 'Play'}
        </button>
        <button
          type="button"
          style={button}
          onClick={() => {
            setPlaying(false);
            setPlayhead(range.to);
          }}
        >
          Jump to end
        </button>
        <label style={{ marginLeft: 8 }}>
          Speed{' '}
          <select
            aria-label="Speed"
            value={speed}
            onChange={(e) => setSpeed(Number(e.target.value))}
            style={{ ...button, padding: '1px 4px' }}
          >
            {SPEEDS.map((s) => (
              <option key={s} value={s}>{`${s}x`}</option>
            ))}
          </select>
        </label>
        <output aria-label="Playhead" style={{ marginLeft: 12, fontFamily: theme.fontMono }}>
          {dateTimeFormat(playhead, { timeZone, format: 'YYYY-MM-DD HH:mm:ss' })}
        </output>
      </div>
      <svg
        viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
        width="100%"
        height={HEIGHT}
        preserveAspectRatio="none"
        aria-hidden="true"
      >
        <line x1={0} y1={HEIGHT - 0.5} x2={WIDTH} y2={HEIGHT - 0.5} stroke={c.viz.axis} strokeWidth={1} />
        <line x1={0} y1={HEIGHT / 2} x2={WIDTH} y2={HEIGHT / 2} stroke={c.viz.grid} strokeWidth={1} />
        {area && <polygon data-testid="timeline-area" points={area} fill={c.viz.areaFill} />}
        {line && (
          <polyline data-testid="timeline-line" points={line} fill="none" stroke={c.viz.series[1]} strokeWidth={1.5} />
        )}
        <line x1={x(playhead)} y1={0} x2={x(playhead)} y2={HEIGHT} stroke={c.text.primary} strokeWidth={1.5} />
      </svg>
      <table style={{ ...st.table, marginTop: 8 }}>
        <thead>
          <tr>
            <th style={st.th}>Signal at the playhead</th>
            <th style={st.th}>Value</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.key}>
              <td style={st.td}>{r.name}</td>
              <td style={{ ...st.td, ...st.mono }}>{r.text}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
