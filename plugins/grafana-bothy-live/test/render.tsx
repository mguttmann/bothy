// SPDX-License-Identifier: Apache-2.0
/** Rendering helper of the view tests: fixed options, period and theme per call. */

import { type DataFrame } from '@grafana/data';
import { render } from '@testing-library/react';
import React from 'react';

import { liveTheme } from '../src/theme';
import { DEFAULT_OPTIONS, type LiveOptions } from '../src/types';
import { type ViewProps } from '../src/views/types';
import { MINUTE, T0 } from './fixtures';

/**
 * Renders a view with fixed frames.
 *
 * @param View - the view component
 * @param frames - the data frames
 * @param opts - dark theme and option overrides
 * @returns the render result of Testing Library
 */
export function renderView(
  View: React.ComponentType<ViewProps>,
  frames: DataFrame[],
  opts: { dark?: boolean; options?: Partial<LiveOptions> } = {}
) {
  const props: ViewProps = {
    frames,
    theme: liveTheme(opts.dark ?? false),
    options: { ...DEFAULT_OPTIONS, ...opts.options },
    range: { from: T0, to: T0 + 10 * MINUTE },
    timeZone: 'utc',
  };
  return render(<View {...props} />);
}
