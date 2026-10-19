// SPDX-License-Identifier: Apache-2.0
import { createDataFrame, FieldType } from '@grafana/data';

import { frame, MINUTE, T0 } from '../test/fixtures';
import { label, lastValue, seriesOf, sumLast, valueAt } from './frames';

describe('frames', () => {
  const frames = [
    frame('active', { bothy_agent_handle: 'Scout', bothy_run_state: 'running' }, [1, 2, null]),
    frame('active', { bothy_agent_handle: 'Lotse', bothy_run_state: 'queued' }, [NaN, 3, 4]),
    frame('cost', { bothy_agent_handle: 'Scout' }, [0.5]),
  ];

  it('reads only the frames of a refId', () => {
    const s = seriesOf(frames, 'active');
    expect(s).toHaveLength(2);
    expect(label(s[0]!, 'bothy_agent_handle')).toBe('Scout');
    expect(label(s[0]!, 'missing')).toBe('');
    expect(seriesOf(frames, 'unknown')).toEqual([]);
  });

  it('turns NaN into no value and finds the last value', () => {
    const [scout, lotse] = seriesOf(frames, 'active');
    expect(lotse!.values[0]).toBeNull();
    expect(lastValue(scout!)).toBe(2);
    expect(lastValue({ labels: {}, times: [], values: [] })).toBeNull();
  });

  it('gives the value at a time', () => {
    const [scout] = seriesOf(frames, 'active');
    expect(valueAt(scout!, T0 - 1)).toBeNull();
    expect(valueAt(scout!, T0 + MINUTE + 5)).toBe(2);
    expect(valueAt(scout!, T0 + 10 * MINUTE)).toBeNull();
  });

  it('adds the last values', () => {
    expect(sumLast(seriesOf(frames, 'active'))).toBe(6);
    expect(sumLast([])).toBeNull();
  });

  it('indexes a frame without a time field by position', () => {
    const f = createDataFrame({ refId: 'x', fields: [{ name: 'Value', type: FieldType.number, values: [7, 8] }] });
    const [s] = seriesOf([f], 'x');
    expect(s!.times).toEqual([0, 1]);
    expect(s!.labels).toEqual({});
  });
});
