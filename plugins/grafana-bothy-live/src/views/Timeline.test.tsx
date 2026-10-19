// SPDX-License-Identifier: Apache-2.0
import { act, fireEvent, screen } from '@testing-library/react';

import { dark, light } from '../../../../design/build/ts/tokens.gen';
import { frame, MINUTE, T0 } from '../../test/fixtures';
import { renderView } from '../../test/render';
import { advance, TICK_MS, TimelineView } from './Timeline';

const frames = [
  frame('active', { bothy_run_state: 'running' }, [1, 3, 2, 0]),
  frame('active', { bothy_run_state: 'queued' }, [0, 1, 1, 0]),
  frame('tool_steps', {}, [4, 8, 6, 0]),
  frame('delivery_p95', {}, [0.05, 0.1, 0.2, 0.05]),
  frame('budget_remaining', {}, [0.9, 0.9, 0.8, 0.8]),
];

describe('advance', () => {
  it('moves the playhead by speed times the tick and stops at the end', () => {
    expect(advance(T0, 60, T0 + 10 * MINUTE)).toBe(T0 + 60 * TICK_MS);
    expect(advance(T0 + 10 * MINUTE - 1, 900, T0 + 10 * MINUTE)).toBe(T0 + 10 * MINUTE);
  });
});

describe('TimelineView', () => {
  afterEach(() => jest.useRealTimers());

  it('starts at the beginning of the dashboard period and shows the values there', () => {
    renderView(TimelineView, frames);
    expect(screen.getByLabelText('Playhead')).toHaveTextContent('2026-10-10 12:00:00');
    expect(screen.getByRole('row', { name: /running/ })).toHaveTextContent('1');
    expect(screen.getByRole('row', { name: /tool steps per minute/ })).toHaveTextContent('4.0/min');
  });

  it('jumps to the end and back', () => {
    renderView(TimelineView, frames);
    fireEvent.click(screen.getByRole('button', { name: 'Jump to end' }));
    expect(screen.getByLabelText('Playhead')).toHaveTextContent('2026-10-10 12:10:00');
    expect(screen.getByRole('row', { name: /lowest remaining error budget/ })).toHaveTextContent('80.00 %');
    fireEvent.click(screen.getByRole('button', { name: 'Jump to start' }));
    expect(screen.getByLabelText('Playhead')).toHaveTextContent('2026-10-10 12:00:00');
  });

  it('plays at the chosen speed and pauses', () => {
    jest.useFakeTimers();
    renderView(TimelineView, frames);
    fireEvent.change(screen.getByLabelText('Speed'), { target: { value: '300' } });
    fireEvent.click(screen.getByRole('button', { name: 'Play' }));
    act(() => {
      jest.advanceTimersByTime(TICK_MS * 4);
    });
    // 4 ticks of 250 ms at 300 s per second: 300 s of time of day.
    expect(screen.getByLabelText('Playhead')).toHaveTextContent('2026-10-10 12:05:00');
    expect(screen.getByRole('row', { name: /delivery p95/ })).toHaveTextContent('50 ms');
    fireEvent.click(screen.getByRole('button', { name: 'Pause' }));
    act(() => {
      jest.advanceTimersByTime(TICK_MS * 8);
    });
    expect(screen.getByLabelText('Playhead')).toHaveTextContent('2026-10-10 12:05:00');
  });

  it('stops at the end of the period', () => {
    jest.useFakeTimers();
    renderView(TimelineView, frames);
    fireEvent.change(screen.getByLabelText('Speed'), { target: { value: '900' } });
    fireEvent.click(screen.getByRole('button', { name: 'Play' }));
    act(() => {
      jest.advanceTimersByTime(TICK_MS * 20);
    });
    expect(screen.getByLabelText('Playhead')).toHaveTextContent('2026-10-10 12:10:00');
    expect(screen.getByRole('button', { name: 'Play' })).toBeInTheDocument();
  });

  it('draws the active runs as an area with the series roles of the theme', () => {
    const { unmount } = renderView(TimelineView, frames);
    expect(screen.getByTestId('timeline-area')).toHaveAttribute('fill', light.viz.areaFill);
    expect(screen.getByTestId('timeline-line')).toHaveAttribute('stroke', light.viz.series[1]);
    unmount();
    renderView(TimelineView, frames, { dark: true });
    expect(screen.getByTestId('timeline-line')).toHaveAttribute('stroke', dark.viz.series[1]);
  });

  it('says so when there is no data', () => {
    renderView(TimelineView, []);
    expect(screen.getByText('No data in this period.')).toBeInTheDocument();
  });
});
