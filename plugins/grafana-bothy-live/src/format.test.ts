// SPDX-License-Identifier: Apache-2.0
import { formatValue, NO_DATA } from './format';

describe('formatValue', () => {
  it.each([
    [3.4, 'count', '3'],
    [12.54, 'per_minute', '12.5/min'],
    [0.12, 'seconds', '120 ms'],
    [1.254, 'seconds', '1.25 s'],
    [0.9995, 'ratio', '99.95 %'],
    [0.4213, 'currency', '0.42'],
  ] as const)('formats %p as %s', (value, unit, text) => {
    expect(formatValue(value, unit)).toBe(text);
  });

  it('says no data for null and NaN', () => {
    expect(formatValue(null, 'count')).toBe(NO_DATA);
    expect(formatValue(Number.NaN, 'ratio')).toBe(NO_DATA);
  });
});
