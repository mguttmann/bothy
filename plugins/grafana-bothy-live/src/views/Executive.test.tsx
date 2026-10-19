// SPDX-License-Identifier: Apache-2.0
import { screen, within } from '@testing-library/react';

import { dark, light } from '../../../../design/build/ts/tokens.gen';
import { frame } from '../../test/fixtures';
import { renderView } from '../../test/render';
import { ExecutiveView, sloState } from './Executive';

function slo(name: string, objective: number | null, errorRatio: number | null, budget: number | null) {
  const labels = { sloth_service: 'bothy-api', sloth_slo: name };
  return [
    ...(objective === null ? [] : [frame('objective', labels, [objective])]),
    ...(errorRatio === null ? [] : [frame('error_ratio', labels, [errorRatio])]),
    ...(budget === null ? [] : [frame('budget_remaining', labels, [budget])]),
  ];
}

const frames = [
  ...slo('requests-availability', 0.999, 0.0002, 0.8),
  ...slo('requests-latency', 0.99, 0.008, 0.2),
  ...slo('delivery-latency', 0.99, 0.02, -1),
  ...slo('push-acceptance', 0.98, null, null),
];

describe('sloState', () => {
  it.each([
    [0.8, 'ok'],
    [0.25, 'ok'],
    [0.2, 'tight'],
    [0, 'missed'],
    [-0.5, 'missed'],
    [null, 'nodata'],
  ] as const)('remaining budget %p is %s', (budget, state) => {
    expect(sloState(budget, 0.25)).toBe(state);
  });
});

describe('ExecutiveView', () => {
  it('shows one traffic light per SLO with icon and text', () => {
    renderView(ExecutiveView, frames);
    const row = (name: string) => screen.getByRole('row', { name: new RegExp(name) });
    expect(within(row('requests-availability')).getByText('on target')).toBeInTheDocument();
    expect(within(row('requests-availability')).getByText('99.98 %')).toBeInTheDocument();
    expect(within(row('requests-availability')).getByText('99.90 %')).toBeInTheDocument();
    expect(within(row('requests-latency')).getByText('budget tight')).toBeInTheDocument();
    expect(within(row('delivery-latency')).getByText('target missed')).toBeInTheDocument();
    expect(within(row('push-acceptance')).getByText('not enough data')).toBeInTheDocument();
    expect(screen.getByText(/Prometheus recording rules/)).toBeInTheDocument();
  });

  it('follows the warn ratio option', () => {
    renderView(ExecutiveView, frames, { options: { warnBudgetRatio: 0.9 } });
    expect(
      within(screen.getByRole('row', { name: /requests-availability/ })).getByText('budget tight')
    ).toBeInTheDocument();
  });

  it('colours the states from the token theme', () => {
    const { unmount } = renderView(ExecutiveView, frames);
    expect(screen.getByTestId('light-requests-availability')).toHaveStyle({ backgroundColor: light.success.fg });
    expect(screen.getByTestId('light-requests-latency')).toHaveStyle({ backgroundColor: light.warning.fg });
    expect(screen.getByTestId('light-delivery-latency')).toHaveStyle({ backgroundColor: light.danger.fg });
    expect(screen.getByTestId('light-push-acceptance')).toHaveStyle({ backgroundColor: light.text.muted });
    unmount();
    renderView(ExecutiveView, frames, { dark: true });
    expect(screen.getByTestId('light-requests-availability')).toHaveStyle({ backgroundColor: dark.success.fg });
  });

  it('says so when there is no SLO', () => {
    renderView(ExecutiveView, []);
    expect(screen.getByText('No SLO recording rules in this period.')).toBeInTheDocument();
  });
});
