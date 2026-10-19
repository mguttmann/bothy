// SPDX-License-Identifier: Apache-2.0
import { dark, light } from '../../../design/build/ts/tokens.gen';
import { liveTheme, cssFontFamily } from './theme';

describe('theme adapter', () => {
  it('takes the dark tokens for a dark Grafana theme and the light ones otherwise', () => {
    expect(liveTheme(true).colors).toBe(dark);
    expect(liveTheme(true).mode).toBe('dark');
    expect(liveTheme(false).colors).toBe(light);
    expect(liveTheme(false).mode).toBe('light');
  });

  it('puts the bundled fonts first', () => {
    expect(liveTheme(false).fontSans.startsWith('"Atkinson Hyperlegible Next", ')).toBe(true);
    expect(liveTheme(false).fontMono.startsWith('"Atkinson Hyperlegible Mono", ')).toBe(true);
  });

  it('quotes only names with a space', () => {
    expect(cssFontFamily(['A B', 'serif'])).toBe('"A B", serif');
  });
});
