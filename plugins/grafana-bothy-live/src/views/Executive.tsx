// SPDX-License-Identifier: Apache-2.0
/**
 * Executive page (docs/spec/25-visualisierung.md 8.2, mirror of 6.4): the SLO traffic light from
 * the Sloth recording rules `slo:objective:ratio`, `slo:sli_error:ratio_rate30d` and
 * `slo:period_error_budget_remaining:ratio`. Never colour alone: every state has an icon and a
 * text (docs/design/components.md section 11).
 */

import type { DataFrame } from '@grafana/data';
import React from 'react';

import { label, lastValue, seriesOf } from '../frames';
import { formatValue } from '../format';
import type { LiveTheme } from '../theme';
import { Dot, tableStyles } from './parts';
import type { ViewProps } from './types';

/** State of one SLO. */
export type SloState = 'ok' | 'tight' | 'missed' | 'nodata';

const STATE: Record<SloState, { text: string; icon: string }> = {
  ok: { text: 'on target', icon: '✓' },
  tight: { text: 'budget tight', icon: '!' },
  missed: { text: 'target missed', icon: '✕' },
  nodata: { text: 'not enough data', icon: '?' },
};

/**
 * The traffic light of a remaining error budget, with the rules of 25 section 6.4.
 *
 * @param budget - remaining error budget ratio of the SLO period, or null without data
 * @param warnRatio - below this the state is "budget tight"
 * @returns ok (at least warnRatio), tight (above 0 and below warnRatio), missed (0 or less) or
 *   nodata (no value: Prometheus has no recording rule result for the SLO)
 */
export function sloState(budget: number | null, warnRatio: number): SloState {
  if (budget === null) {
    return 'nodata';
  }
  if (budget <= 0) {
    return 'missed';
  }
  return budget < warnRatio ? 'tight' : 'ok';
}

/**
 * The token colour of a state.
 *
 * @param state - the SLO state
 * @param theme - the token theme
 * @returns success, warning or danger foreground; the muted text role for no data
 */
export function stateColor(state: SloState, theme: LiveTheme): string {
  const c = theme.colors;
  return { ok: c.success.fg, tight: c.warning.fg, missed: c.danger.fg, nodata: c.text.muted }[state];
}

/** One SLO with its three values. */
export interface SloRow {
  readonly service: string;
  readonly slo: string;
  readonly objective: number | null;
  readonly errorRatio: number | null;
  readonly budget: number | null;
}

/**
 * The SLOs of the frames of the executive queries (refIds `objective`, `error_ratio`,
 * `budget_remaining`), keyed by `sloth_service` and `sloth_slo`.
 *
 * @param frames - data frames of the panel
 * @returns one row per SLO, sorted by service and name
 */
export function executiveModel(frames: readonly DataFrame[]): SloRow[] {
  const rows = new Map<
    string,
    { service: string; slo: string; objective: number | null; errorRatio: number | null; budget: number | null }
  >();
  const read = (refId: string, key: 'objective' | 'errorRatio' | 'budget') => {
    for (const s of seriesOf(frames, refId)) {
      const service = label(s, 'sloth_service');
      const slo = label(s, 'sloth_slo');
      const id = `${service}\u0000${slo}`;
      const row = rows.get(id) ?? { service, slo, objective: null, errorRatio: null, budget: null };
      row[key] = lastValue(s);
      rows.set(id, row);
    }
  };
  read('objective', 'objective');
  read('error_ratio', 'errorRatio');
  read('budget_remaining', 'budget');
  return [...rows.values()].sort((a, b) => a.service.localeCompare(b.service) || a.slo.localeCompare(b.slo));
}

/**
 * The executive page.
 *
 * @param props - frames, theme and options (warnBudgetRatio)
 * @returns the rendering
 */
export function ExecutiveView({ frames, theme, options }: ViewProps) {
  const rows = executiveModel(frames);
  const st = tableStyles(theme);
  if (rows.length === 0) {
    return <p style={st.empty}>No SLO recording rules in this period.</p>;
  }
  return (
    <>
      <table style={st.table}>
        <thead>
          <tr>
            <th style={st.th}>SLO</th>
            <th style={st.th}>State</th>
            <th style={st.th}>Good share (30 days)</th>
            <th style={st.th}>Objective</th>
            <th style={st.th}>Remaining budget</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => {
            const state =
              r.objective === null || r.errorRatio === null ? 'nodata' : sloState(r.budget, options.warnBudgetRatio);
            const color = stateColor(state, theme);
            return (
              <tr key={`${r.service}/${r.slo}`}>
                <td style={st.td}>
                  <div style={{ fontWeight: 600 }}>{r.slo}</div>
                  <div style={{ color: theme.colors.text.muted, fontSize: theme.sizes[12] }}>{r.service}</div>
                </td>
                <td style={{ ...st.td, whiteSpace: 'nowrap' }}>
                  <Dot color={color} testId={`light-${r.slo}`} />
                  <span aria-hidden="true" style={{ color, fontWeight: 700, marginRight: 4 }}>
                    {STATE[state].icon}
                  </span>
                  <span>{STATE[state].text}</span>
                </td>
                <td style={{ ...st.td, ...st.mono }}>
                  {formatValue(r.errorRatio === null ? null : 1 - r.errorRatio, 'ratio')}
                </td>
                <td style={{ ...st.td, ...st.mono }}>{formatValue(r.objective, 'ratio')}</td>
                <td style={{ ...st.td, ...st.mono }}>{formatValue(r.budget, 'ratio')}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
      <p style={st.note}>
        From the Prometheus recording rules of the Sloth files; values can differ from the own measurement of the apps.
        Burn-rate alerts stay in Alertmanager.
      </p>
    </>
  );
}
