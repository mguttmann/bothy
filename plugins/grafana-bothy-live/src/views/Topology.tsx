// SPDX-License-Identifier: Apache-2.0
/**
 * Live topology (docs/spec/25-visualization.md 8.2, mirror of 6.1): one lane per agent with its
 * active runs by state, tool steps per minute and costs. Metrics carry no IDs (17 rule 9), so a
 * lane is an agent, never a channel or a single run.
 */

import type { DataFrame } from '@grafana/data';
import React from 'react';

import { label, lastValue, seriesOf } from '../frames';
import { formatValue } from '../format';
import { Dot, tableStyles } from './parts';
import type { ViewProps } from './types';

/** Run states of `bothy.run.active` (signals.md), in lane order. */
export const RUN_STATES = ['running', 'waiting_input', 'queued'] as const;
/** One run state. */
export type RunState = (typeof RUN_STATES)[number];

const STATE_TEXT: Record<RunState, string> = {
  running: 'running',
  waiting_input: 'waiting for input',
  queued: 'queued',
};

/** Cost of an agent in one currency over the dashboard period. */
export interface Cost {
  readonly currency: string;
  readonly amount: number;
  /** True when the adapter reported an estimate (`bothy.cost.estimate`). */
  readonly estimate: boolean;
}

/** One lane: an agent with its numbers. */
export interface Lane extends Record<RunState, number> {
  readonly agent: string;
  readonly toolStepsPerMinute: number | null;
  readonly costs: Cost[];
}

/**
 * The lanes of the frames of the topology queries (refIds `active`, `tool_steps`, `cost`).
 *
 * @param frames - data frames of the panel
 * @returns one lane per agent handle that has a value, sorted by handle
 */
export function topologyModel(frames: readonly DataFrame[]): Lane[] {
  const lanes = new Map<
    string,
    { agent: string; toolStepsPerMinute: number | null; costs: Cost[] } & Record<RunState, number>
  >();
  const lane = (agent: string) => {
    let l = lanes.get(agent);
    if (!l) {
      l = { agent, running: 0, waiting_input: 0, queued: 0, toolStepsPerMinute: null, costs: [] };
      lanes.set(agent, l);
    }
    return l;
  };
  for (const s of seriesOf(frames, 'active')) {
    const state = label(s, 'bothy_run_state') as RunState;
    const v = lastValue(s);
    if (v !== null && RUN_STATES.includes(state)) {
      lane(label(s, 'bothy_agent_handle'))[state] += v;
    }
  }
  for (const s of seriesOf(frames, 'tool_steps')) {
    const v = lastValue(s);
    if (v !== null) {
      lane(label(s, 'bothy_agent_handle')).toolStepsPerMinute = v;
    }
  }
  for (const s of seriesOf(frames, 'cost')) {
    const v = lastValue(s);
    if (v !== null && v > 0) {
      lane(label(s, 'bothy_agent_handle')).costs.push({
        currency: label(s, 'bothy_currency'),
        amount: v,
        estimate: label(s, 'bothy_cost_estimate') === 'true',
      });
    }
  }
  return [...lanes.values()].sort((a, b) => a.agent.localeCompare(b.agent));
}

/**
 * Text of a cost, with "estimate" when the adapter only estimated it.
 *
 * @param c - the cost
 * @returns for example `0.42 USD estimate`
 */
export function costText(c: Cost): string {
  return `${formatValue(c.amount, 'currency')} ${c.currency}${c.estimate ? ' estimate' : ''}`;
}

/**
 * The live topology view.
 *
 * @param props - frames, theme and options
 * @returns the rendering
 */
export function TopologyView({ frames, theme, options }: ViewProps) {
  const lanes = topologyModel(frames);
  const st = tableStyles(theme);
  if (lanes.length === 0) {
    return <p style={st.empty}>No agent activity in this period.</p>;
  }
  const total = lanes.reduce((n, l) => n + l.running + l.waiting_input + l.queued, 0);
  const stepColor = (state: RunState) =>
    state === 'waiting_input' ? theme.colors.run.waiting : theme.colors.run[state];
  return (
    <>
      <table style={st.table}>
        <thead>
          <tr>
            <th style={st.th}>Agent</th>
            <th style={st.th}>Active runs</th>
            <th style={st.th}>Tool steps</th>
            {options.showCosts && <th style={st.th}>Cost</th>}
          </tr>
        </thead>
        <tbody>
          {lanes.map((l) => {
            const lead = RUN_STATES.find((s) => l[s] > 0);
            return (
              <tr key={l.agent}>
                <td
                  style={{
                    ...st.td,
                    borderLeft: `3px solid ${lead ? stepColor(lead) : theme.colors.viz.grid}`,
                    fontWeight: 600,
                  }}
                >
                  {l.agent}
                </td>
                <td style={st.td}>
                  {RUN_STATES.filter((s) => l[s] > 0).map((s) => (
                    <span key={s} style={{ marginRight: 12, whiteSpace: 'nowrap' }}>
                      <Dot color={stepColor(s)} testId={`state-dot-${l.agent}-${s}`} />
                      <span>{`${STATE_TEXT[s]} ${l[s]}`}</span>
                    </span>
                  ))}
                </td>
                <td style={{ ...st.td, ...st.mono }}>{formatValue(l.toolStepsPerMinute, 'per_minute')}</td>
                {options.showCosts && (
                  <td style={{ ...st.td, ...st.mono }}>
                    {l.costs.map((c) => (
                      <div key={c.currency + String(c.estimate)}>{costText(c)}</div>
                    ))}
                  </td>
                )}
              </tr>
            );
          })}
        </tbody>
      </table>
      <p style={st.note}>{`Active runs: ${total}`}</p>
    </>
  );
}
