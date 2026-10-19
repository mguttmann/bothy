// SPDX-License-Identifier: Apache-2.0
import { createTheme, getDefaultTimeRange, LoadingState, type PanelProps, ThemeContext } from '@grafana/data';
import { render, screen } from '@testing-library/react';
import React from 'react';

import { dark, light } from '../../../../design/build/ts/tokens.gen';
import { frame } from '../../test/fixtures';
import { DEFAULT_OPTIONS, type LiveOptions } from '../types';
import { LivePanel } from './LivePanel';

function panel(options: Partial<LiveOptions>, mode: 'light' | 'dark') {
  const timeRange = getDefaultTimeRange();
  const props = {
    id: 1,
    data: {
      state: LoadingState.Done,
      series: [frame('active', { bothy_agent_handle: 'Scout', bothy_run_state: 'running' }, [1])],
      timeRange,
    },
    timeRange,
    timeZone: 'utc',
    options: { ...DEFAULT_OPTIONS, ...options },
    width: 800,
    height: 400,
    title: 'Live',
  } as unknown as PanelProps<LiveOptions>;
  return render(
    <ThemeContext.Provider value={createTheme({ colors: { mode } })}>
      <LivePanel {...props} />
    </ThemeContext.Provider>
  );
}

describe('LivePanel', () => {
  it.each([
    ['topology', 'Live topology'],
    ['backend', 'Backend view'],
    ['delivery', 'Delivery and sync'],
    ['executive', 'Executive page'],
    ['timeline', 'Timeline'],
  ] as const)('renders the view %s', (view, title) => {
    panel({ view }, 'light');
    expect(screen.getByRole('region', { name: title })).toBeInTheDocument();
  });

  it('follows the Grafana theme with the token surfaces and the bundled font', () => {
    const { unmount } = panel({ view: 'topology' }, 'light');
    const region = screen.getByRole('region', { name: 'Live topology' });
    expect(region).toHaveStyle({ backgroundColor: light.surface.canvas, color: light.text.primary });
    expect(region.style.fontFamily).toContain('Atkinson Hyperlegible Next');
    expect(region).toHaveAttribute('data-theme', 'light');
    unmount();
    panel({ view: 'topology' }, 'dark');
    const darkRegion = screen.getByRole('region', { name: 'Live topology' });
    expect(darkRegion).toHaveStyle({ backgroundColor: dark.surface.canvas, color: dark.text.primary });
    expect(darkRegion).toHaveAttribute('data-theme', 'dark');
  });

  it('registers the bundled fonts once', () => {
    panel({ view: 'topology' }, 'light');
    panel({ view: 'backend' }, 'light');
    const styles = document.head.querySelectorAll('style[data-bothy-live-fonts]');
    expect(styles).toHaveLength(1);
    expect(styles[0]!.textContent).toContain("font-family: 'Atkinson Hyperlegible Next'");
    expect(styles[0]!.textContent).toContain("font-family: 'Atkinson Hyperlegible Mono'");
  });
});
