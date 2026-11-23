// SPDX-License-Identifier: Apache-2.0
/**
 * The panel component: picks the view of the panel options and wraps it in the token theme that
 * follows Grafana's light or dark theme. The panel reads only the frames Grafana's data sources
 * return for its targets; it never talks to the product server (docs/spec/25-visualization.md 8.1).
 * A failed query shows as a failure and a first load as loading, so neither looks like a quiet period.
 */

import { LoadingState, type PanelProps } from '@grafana/data';
import React, { useEffect } from 'react';

import { ensureFonts } from '../fonts';
import { QUERIES, type ViewName } from '../queries.gen';
import { useLiveTheme } from '../theme';
import type { LiveOptions } from '../types';
import { BackendView } from '../views/Backend';
import { DeliveryView } from '../views/Delivery';
import { ExecutiveView } from '../views/Executive';
import { TimelineView } from '../views/Timeline';
import { TopologyView } from '../views/Topology';
import type { ViewProps } from '../views/types';

const VIEWS: Record<ViewName, React.ComponentType<ViewProps>> = {
  topology: TopologyView,
  backend: BackendView,
  delivery: DeliveryView,
  executive: ExecutiveView,
  timeline: TimelineView,
};

/**
 * Panel of the plugin bothy-live-panel.
 *
 * @param props - Grafana's panel props; `options.view` selects the view
 * @returns the rendering
 */
export function LivePanel({ data, options, width, height, timeRange, timeZone }: PanelProps<LiveOptions>) {
  const theme = useLiveTheme();
  useEffect(() => ensureFonts(), []);
  const View = VIEWS[options.view] ?? TopologyView;
  const view = VIEWS[options.view] ? options.view : 'topology';
  const failed = data.state === LoadingState.Error;
  const errorText = (data.errors ?? [])
    .map((e) => e.message)
    .filter((m): m is string => typeof m === 'string' && m.length > 0)
    .join('; ');
  const loading = data.state === LoadingState.Loading && data.series.length === 0;
  // The empty texts of the views mean "no activity" and hold only when every query returned.
  const showView = !loading && (!failed || data.series.length > 0);
  return (
    <section
      aria-label={QUERIES[view].title}
      data-theme={theme.mode}
      style={{
        width,
        height,
        overflow: 'auto',
        boxSizing: 'border-box',
        padding: 8,
        backgroundColor: theme.colors.surface.canvas,
        color: theme.colors.text.primary,
        fontFamily: theme.fontSans,
        fontSize: theme.sizes[14],
        borderRadius: theme.radii.md,
      }}
    >
      {failed && (
        <p role="alert" style={{ color: theme.colors.danger.fg, fontWeight: 600, margin: '0 0 8px' }}>
          {`Query failed: ${errorText || 'no message'}`}
        </p>
      )}
      {loading && <p style={{ color: theme.colors.text.muted, margin: 0 }}>Loading</p>}
      {showView && (
        <View
          frames={data.series}
          theme={theme}
          options={options}
          range={{ from: timeRange.from.valueOf(), to: timeRange.to.valueOf() }}
          timeZone={timeZone}
        />
      )}
    </section>
  );
}
