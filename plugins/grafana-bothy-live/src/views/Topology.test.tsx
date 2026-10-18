// SPDX-License-Identifier: Apache-2.0
import { screen, within } from '@testing-library/react';

import { dark, light } from '../../../../design/build/ts/tokens.gen';
import { frame } from '../../test/fixtures';
import { renderView } from '../../test/render';
import { TopologyView, topologyModel } from './Topology';

const frames = [
  frame('active', { bothy_agent_handle: 'Scout', bothy_run_state: 'running' }, [2]),
  frame('active', { bothy_agent_handle: 'Scout', bothy_run_state: 'waiting_input' }, [1]),
  frame('active', { bothy_agent_handle: 'Lotse', bothy_run_state: 'queued' }, [1]),
  frame('active', { bothy_agent_handle: 'Lotse', bothy_run_state: 'running' }, [0]),
  frame('tool_steps', { bothy_agent_handle: 'Scout' }, [12.5]),
  frame('cost', { bothy_agent_handle: 'Scout', bothy_currency: 'USD', bothy_cost_estimate: 'true' }, [0.42]),
  frame('cost', { bothy_agent_handle: 'Lotse', bothy_currency: 'EUR', bothy_cost_estimate: 'false' }, [1.5]),
  frame('other', { bothy_agent_handle: 'Ignored' }, [9]),
];

describe('TopologyView', () => {
  it('builds one lane per agent, sorted by handle', () => {
    const lanes = topologyModel(frames);
    expect(lanes.map((l) => l.agent)).toEqual(['Lotse', 'Scout']);
    expect(lanes[1]).toMatchObject({ queued: 0, running: 2, waiting_input: 1, toolStepsPerMinute: 12.5 });
    expect(lanes[1]!.costs).toEqual([{ currency: 'USD', amount: 0.42, estimate: true }]);
  });

  it('renders lanes with states, tool steps and costs', () => {
    renderView(TopologyView, frames);
    const scout = screen.getByRole('row', { name: /Scout/ });
    expect(within(scout).getByText('running 2')).toBeInTheDocument();
    expect(within(scout).getByText('waiting for input 1')).toBeInTheDocument();
    expect(within(scout).getByText('12.5/min')).toBeInTheDocument();
    expect(within(scout).getByText('0.42 USD estimate')).toBeInTheDocument();
    const lotse = screen.getByRole('row', { name: /Lotse/ });
    expect(within(lotse).getByText('queued 1')).toBeInTheDocument();
    expect(within(lotse).queryByText(/running/)).toBeNull();
    expect(within(lotse).getByText('1.50 EUR')).toBeInTheDocument();
    expect(screen.queryByText('Ignored')).toBeNull();
    expect(screen.getByText('Active runs: 4')).toBeInTheDocument();
  });

  it('hides costs when the option is off', () => {
    renderView(TopologyView, frames, { options: { showCosts: false } });
    expect(screen.queryByText(/USD/)).toBeNull();
    expect(screen.queryByRole('columnheader', { name: 'Cost' })).toBeNull();
  });

  it('says so when there is no data', () => {
    renderView(TopologyView, []);
    expect(screen.getByText('No agent activity in this period.')).toBeInTheDocument();
  });

  it('colours the state chips from the token theme of Grafana', () => {
    const { unmount } = renderView(TopologyView, frames);
    expect(screen.getByTestId('state-dot-Scout-running')).toHaveStyle({ backgroundColor: light.run.running });
    unmount();
    renderView(TopologyView, frames, { dark: true });
    expect(screen.getByTestId('state-dot-Scout-running')).toHaveStyle({ backgroundColor: dark.run.running });
  });
});
