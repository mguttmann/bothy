// SPDX-License-Identifier: Apache-2.0
import { screen, within } from '@testing-library/react';

import { dark, light } from '../../../../design/build/ts/tokens.gen';
import { frame } from '../../test/fixtures';
import { renderView } from '../../test/render';
import { BackendView, backendModel } from './Backend';

const frames = [
  frame('mode_runs', { bothy_agent_handle: 'Scout', bothy_billing_mode: 'organization' }, [7]),
  frame('mode_runs', { bothy_agent_handle: 'Scout', bothy_billing_mode: 'personal' }, [2]),
  frame('mode_runs', { bothy_agent_handle: 'Lotse', bothy_billing_mode: 'personal' }, [3]),
  frame('mismatches', { bothy_agent_handle: 'Scout' }, [1]),
  frame('denials', { bothy_agent_handle: 'Lotse', bothy_reason: 'i2_denied' }, [4]),
  frame(
    'cost_by_mode',
    {
      bothy_agent_handle: 'Scout',
      bothy_billing_mode: 'organization',
      bothy_currency: 'USD',
      bothy_cost_estimate: 'true',
    },
    [2.5]
  ),
];

describe('BackendView', () => {
  it('collects modes, mismatches, denials and costs per agent', () => {
    const rows = backendModel(frames);
    expect(rows.map((r) => r.agent)).toEqual(['Lotse', 'Scout']);
    expect(rows[1]).toMatchObject({ modes: { organization: 7, personal: 2 }, mismatches: 1 });
    expect(rows[0]).toMatchObject({
      modes: { personal: 3 },
      mismatches: 0,
      denials: [{ reason: 'i2_denied', count: 4 }],
    });
  });

  it('renders one row per agent', () => {
    renderView(BackendView, frames);
    const scout = screen.getByRole('row', { name: /Scout/ });
    expect(within(scout).getByText('organization 7')).toBeInTheDocument();
    expect(within(scout).getByText('personal 2')).toBeInTheDocument();
    expect(within(scout).getByText('1 mismatch')).toBeInTheDocument();
    expect(within(scout).getByText('organization 2.50 USD estimate')).toBeInTheDocument();
    const lotse = screen.getByRole('row', { name: /Lotse/ });
    expect(within(lotse).getByText('i2_denied 4')).toBeInTheDocument();
    expect(within(lotse).getByText('none')).toBeInTheDocument();
    expect(screen.getByText(/Rate-limit state is shown only in the apps/)).toBeInTheDocument();
  });

  it('hides costs when the option is off', () => {
    renderView(BackendView, frames, { options: { showCosts: false } });
    expect(screen.queryByText(/USD/)).toBeNull();
  });

  it('says so when there is no data', () => {
    renderView(BackendView, []);
    expect(screen.getByText('No runs in this period.')).toBeInTheDocument();
  });

  it('marks a mismatch with the danger role of the token theme', () => {
    const { unmount } = renderView(BackendView, frames);
    expect(screen.getByText('1 mismatch')).toHaveStyle({ color: light.danger.fg });
    unmount();
    renderView(BackendView, frames, { dark: true });
    expect(screen.getByText('1 mismatch')).toHaveStyle({ color: dark.danger.fg });
  });
});
