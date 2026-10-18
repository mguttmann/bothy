// SPDX-License-Identifier: Apache-2.0
import { screen } from '@testing-library/react';

import { dark, light } from '../../../../design/build/ts/tokens.gen';
import { frame } from '../../test/fixtures';
import { renderView } from '../../test/render';
import { DeliveryView } from './Delivery';

const frames = [
  frame('delivery_p95', {}, [0.05, 0.08, 0.12]),
  frame('connect_p95', {}, [0.5, 1.25]),
  frame('catchup_p95', {}, [2, 3]),
  frame('fanout_lag', {}, [0, 0]),
];

describe('DeliveryView', () => {
  it('shows the latest value of each signal', () => {
    renderView(DeliveryView, frames);
    expect(screen.getByRole('group', { name: 'delivery p95' })).toHaveTextContent('120 ms');
    expect(screen.getByRole('group', { name: 'connect p95' })).toHaveTextContent('1.25 s');
    expect(screen.getByRole('group', { name: 'catch-up p95' })).toHaveTextContent('3.00 s');
    expect(screen.getByRole('group', { name: 'fan-out lag' })).toHaveTextContent('0 ms');
  });

  it('draws a sparkline per signal with a series role', () => {
    const { unmount } = renderView(DeliveryView, frames);
    expect(screen.getByTestId('spark-delivery_p95')).toHaveAttribute('stroke', light.viz.series[1]);
    unmount();
    renderView(DeliveryView, frames, { dark: true });
    expect(screen.getByTestId('spark-delivery_p95')).toHaveAttribute('stroke', dark.viz.series[1]);
  });

  it('shows no data per tile without frames', () => {
    renderView(DeliveryView, []);
    expect(screen.getAllByText('no data')).toHaveLength(4);
    expect(screen.queryByTestId('spark-delivery_p95')).toBeNull();
  });
});
