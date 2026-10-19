// SPDX-License-Identifier: Apache-2.0
/** The contract of every view: data frames and theme in, a rendering out. */

import type { DataFrame } from '@grafana/data';

import type { LiveTheme } from '../theme';
import type { LiveOptions } from '../types';

/** Props of a view component. */
export interface ViewProps {
  /** Frames of the panel's queries; a view reads only the refIds of its queries.yaml entry. */
  readonly frames: readonly DataFrame[];
  /** Token theme that follows Grafana's theme. */
  readonly theme: LiveTheme;
  /** Panel options. */
  readonly options: LiveOptions;
  /** The dashboard period in epoch milliseconds. */
  readonly range: { readonly from: number; readonly to: number };
  /** Grafana time zone of the dashboard (for example `browser` or `utc`). */
  readonly timeZone: string;
}
