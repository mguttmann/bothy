// SPDX-License-Identifier: Apache-2.0
/**
 * Backend view (docs/spec/25-visualization.md 8.2, mirror of 6.2): per agent the billing modes
 * its runs started in, I3 mismatches, I2 denials and costs per mode. Product-neutral: no runtime
 * names, no subscriptions, no credentials (I5); the rate-limit state exists only in level a.
 */

import type { DataFrame } from '@grafana/data';
import React from 'react';

import { label, lastValue, seriesOf } from '../frames';
import { formatValue } from '../format';
import { tableStyles } from './parts';
import { costText, type Cost } from './Topology';
import type { ViewProps } from './types';

/** One row of the backend view. */
export interface BackendRow {
  readonly agent: string;
  /** Runs started per billing mode over the dashboard period. */
  readonly modes: Record<string, number>;
  /** Runs failed with `billing_mode_mismatch` (I3). */
  readonly mismatches: number;
  /** Denials before delivery (I2) by reason. */
  readonly denials: Array<{ reason: string; count: number }>;
  /** Costs per mode. */
  readonly costs: Array<Cost & { mode: string }>;
}

/**
 * The rows of the frames of the backend queries (refIds `mode_runs`, `mismatches`, `denials`,
 * `cost_by_mode`).
 *
 * @param frames - data frames of the panel
 * @returns one row per agent handle, sorted by handle; zero counts are left out
 */
export function backendModel(frames: readonly DataFrame[]): BackendRow[] {
  const rows = new Map<
    string,
    {
      agent: string;
      modes: Record<string, number>;
      mismatches: number;
      denials: BackendRow['denials'];
      costs: BackendRow['costs'];
    }
  >();
  const row = (agent: string) => {
    let r = rows.get(agent);
    if (!r) {
      r = { agent, modes: {}, mismatches: 0, denials: [], costs: [] };
      rows.set(agent, r);
    }
    return r;
  };
  const each = (refId: string, fn: (agent: string, value: number, s: ReturnType<typeof seriesOf>[number]) => void) => {
    for (const s of seriesOf(frames, refId)) {
      const v = lastValue(s);
      if (v !== null && Math.round(v * 100) > 0) {
        fn(label(s, 'bothy_agent_handle'), v, s);
      }
    }
  };
  each('mode_runs', (agent, v, s) => {
    const r = row(agent);
    const mode = label(s, 'bothy_billing_mode');
    r.modes[mode] = (r.modes[mode] ?? 0) + Math.round(v);
  });
  each('mismatches', (agent, v) => {
    row(agent).mismatches += Math.round(v);
  });
  each('denials', (agent, v, s) => {
    row(agent).denials.push({ reason: label(s, 'bothy_reason'), count: Math.round(v) });
  });
  each('cost_by_mode', (agent, v, s) => {
    row(agent).costs.push({
      mode: label(s, 'bothy_billing_mode'),
      currency: label(s, 'bothy_currency'),
      amount: v,
      estimate: label(s, 'bothy_cost_estimate') === 'true',
    });
  });
  return [...rows.values()].sort((a, b) => a.agent.localeCompare(b.agent));
}

/**
 * The backend view.
 *
 * @param props - frames, theme and options
 * @returns the rendering
 */
export function BackendView({ frames, theme, options }: ViewProps) {
  const rows = backendModel(frames);
  const st = tableStyles(theme);
  if (rows.length === 0) {
    return <p style={st.empty}>No runs in this period.</p>;
  }
  return (
    <>
      <table style={st.table}>
        <thead>
          <tr>
            <th style={st.th}>Agent</th>
            <th style={st.th}>Runs per mode</th>
            <th style={st.th}>Mode mismatches</th>
            <th style={st.th}>Denials</th>
            {options.showCosts && <th style={st.th}>Cost per mode</th>}
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.agent}>
              <td style={{ ...st.td, fontWeight: 600 }}>{r.agent}</td>
              <td style={st.td}>
                {Object.entries(r.modes)
                  .sort(([a], [b]) => a.localeCompare(b))
                  .map(([mode, n]) => (
                    <div key={mode}>{`${mode} ${n}`}</div>
                  ))}
              </td>
              <td style={st.td}>
                {r.mismatches > 0 ? (
                  <span style={{ color: theme.colors.danger.fg, fontWeight: 600 }}>
                    {`${r.mismatches} ${r.mismatches === 1 ? 'mismatch' : 'mismatches'}`}
                  </span>
                ) : (
                  <span style={{ color: theme.colors.text.muted }}>none</span>
                )}
              </td>
              <td style={st.td}>
                {r.denials.map((d) => (
                  <div key={d.reason} style={{ color: theme.colors.warning.fg }}>{`${d.reason} ${d.count}`}</div>
                ))}
              </td>
              {options.showCosts && (
                <td style={{ ...st.td, ...st.mono }}>
                  {r.costs.map((c) => (
                    <div key={c.mode + c.currency + String(c.estimate)}>{`${c.mode} ${costText(c)}`}</div>
                  ))}
                </td>
              )}
            </tr>
          ))}
        </tbody>
      </table>
      <p style={st.note}>
        {`Counts over the dashboard period (${formatValue(rows.length, 'count')} agents). Rate-limit state is shown only in the apps.`}
      </p>
    </>
  );
}
