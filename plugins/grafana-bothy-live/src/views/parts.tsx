// SPDX-License-Identifier: Apache-2.0
/** Building blocks shared by the views: table styles, a coloured dot and a sparkline. */

import React from 'react';

import type { LiveTheme } from '../theme';

/**
 * Styles of a data table in the token theme. Every view shows its values as a real table, so a
 * screen reader gets the same numbers (docs/design/components.md section 11).
 *
 * @param theme - the token theme
 * @returns inline styles for table, header cell, cell and note
 */
export function tableStyles(theme: LiveTheme) {
  const c = theme.colors;
  return {
    table: { width: '100%', borderCollapse: 'collapse', fontSize: theme.sizes[14] } as React.CSSProperties,
    th: {
      textAlign: 'left',
      fontWeight: 600,
      color: c.text.secondary,
      fontSize: theme.sizes[12],
      padding: '4px 8px',
      borderBottom: `1px solid ${c.border.default}`,
    } as React.CSSProperties,
    td: { padding: '6px 8px', borderBottom: `1px solid ${c.viz.grid}`, verticalAlign: 'top' } as React.CSSProperties,
    mono: { fontFamily: theme.fontMono } as React.CSSProperties,
    note: { color: c.text.muted, fontSize: theme.sizes[12], margin: '8px 0 0' } as React.CSSProperties,
    empty: { color: c.text.muted, margin: 0 } as React.CSSProperties,
  };
}

/** Props of Dot. */
export interface DotProps {
  /** Fill colour, a token value. */
  readonly color: string;
  /** Test id; the tests read the colour from it. */
  readonly testId?: string;
}

/**
 * A small coloured circle. It never carries meaning alone: a text always stands next to it.
 *
 * @param props - colour and test id
 * @returns the element
 */
export function Dot({ color, testId }: DotProps) {
  return (
    <span
      aria-hidden="true"
      data-testid={testId}
      style={{
        display: 'inline-block',
        width: 8,
        height: 8,
        borderRadius: '50%',
        backgroundColor: color,
        marginRight: 4,
      }}
    />
  );
}

/** Props of Sparkline. */
export interface SparklineProps {
  /** Values in time order; null points are skipped. */
  readonly values: ReadonlyArray<number | null>;
  /** Stroke colour, a token value. */
  readonly color: string;
  /** Test id of the line. */
  readonly testId?: string;
  /** Width and height in px. */
  readonly width?: number;
  readonly height?: number;
}

/**
 * A line of the values over time without axes; nothing when fewer than two points have values.
 *
 * @param props - values, colour and size
 * @returns the SVG element or null
 */
export function Sparkline({ values, color, testId, width = 120, height = 28 }: SparklineProps) {
  const points = values.flatMap((v, i) => (v === null ? [] : [[i, v] as const]));
  if (points.length < 2) {
    return null;
  }
  const max = Math.max(...points.map(([, v]) => v));
  const min = Math.min(...points.map(([, v]) => v));
  const span = max - min || 1;
  const last = values.length - 1 || 1;
  const d = points
    .map(([i, v]) => `${((i / last) * width).toFixed(1)},${(height - ((v - min) / span) * height).toFixed(1)}`)
    .join(' ');
  return (
    <svg width={width} height={height} aria-hidden="true">
      <polyline data-testid={testId} points={d} fill="none" stroke={color} strokeWidth={1.5} />
    </svg>
  );
}
