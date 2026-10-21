// SPDX-License-Identifier: Apache-2.0
/**
 * Theme adapter: Grafana's light or dark theme selects the matching theme of the design tokens
 * (docs/spec/25-visualization.md section 10 items 3 and 4).
 *
 * Colours, font families, sizes and radii come only from design/build/ts/tokens.gen.ts, the
 * TypeScript output of tools/design-tokens. Grafana's own palette is never used, so the panel
 * looks like the apps; the lint rule `bothy/no-color-literal` keeps colour literals out of src/.
 */

import { useTheme2 } from '@grafana/ui';

import { dark, fonts, light, radii, sizes, type Theme } from '../../../design/build/ts/tokens.gen';

/** Everything a view needs to style itself. */
export interface LiveTheme {
  /** Which token theme is active. */
  readonly mode: 'light' | 'dark';
  /** Colour roles of the active token theme. */
  readonly colors: Theme;
  /** CSS font-family value of the sans family (Atkinson Hyperlegible Next first). */
  readonly fontSans: string;
  /** CSS font-family value of the mono family (Atkinson Hyperlegible Mono first). */
  readonly fontMono: string;
  /** Font sizes in px. */
  readonly sizes: typeof sizes;
  /** Corner radii in px. */
  readonly radii: typeof radii;
}

/**
 * Renders a token font list as a CSS font-family value; names with a space are quoted.
 *
 * @param families - the family names, preferred first
 * @returns the value for `font-family`
 * @example cssFontFamily(['Atkinson Hyperlegible Next', 'sans-serif']) is
 *   `"Atkinson Hyperlegible Next", sans-serif`
 */
export function cssFontFamily(families: readonly string[]): string {
  return families.map((f) => (/\s/.test(f) ? JSON.stringify(f) : f)).join(', ');
}

/**
 * The token theme for Grafana's mode.
 *
 * @param isDark - whether Grafana shows its dark theme
 * @returns the dark token theme for true, the light one for false
 */
export function liveTheme(isDark: boolean): LiveTheme {
  return {
    mode: isDark ? 'dark' : 'light',
    colors: isDark ? dark : light,
    fontSans: cssFontFamily(fonts.sans),
    fontMono: cssFontFamily(fonts.mono),
    sizes,
    radii,
  };
}

/**
 * React hook: the token theme that follows Grafana's current theme.
 *
 * @returns the token theme for `useTheme2().isDark`
 */
export function useLiveTheme(): LiveTheme {
  return liveTheme(useTheme2().isDark);
}
