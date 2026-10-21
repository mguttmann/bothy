// SPDX-License-Identifier: Apache-2.0
/** Options of the panel bothy-live-panel, set per panel in the dashboard JSON or the editor. */

import type { ViewName } from './queries.gen';

/** Options of one panel. */
export interface LiveOptions {
  /** Which of the five views of docs/spec/25-visualization.md 8.2 this panel shows. */
  view: ViewName;
  /**
   * Executive page: below this remaining error budget the traffic light is "budget tight".
   * Same meaning and default as `stats.slo_warn_budget_ratio` of the apps (25 section 15).
   */
  warnBudgetRatio: number;
  /** Live topology and backend view: show costs (on by default; off for a screen in a room). */
  showCosts: boolean;
}

/** Defaults of every option. */
export const DEFAULT_OPTIONS: LiveOptions = {
  view: 'topology',
  warnBudgetRatio: 0.25,
  showCosts: true,
};
