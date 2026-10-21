// SPDX-License-Identifier: Apache-2.0
/**
 * Registers the bundled Atkinson Hyperlegible fonts (OFL-1.1) once per page
 * (docs/spec/25-visualization.md section 10 item 4). The files are the copies of
 * design/build/ts/fonts/ that tools/design-tokens keeps; webpack emits them next to module.js
 * and gives their URL, and webpack.config.ts copies the licence texts beside them.
 */

import monoLatinExt from '../../../design/build/ts/fonts/atkinson-hyperlegible-mono-latin-ext-wght-normal.woff2';
import monoLatin from '../../../design/build/ts/fonts/atkinson-hyperlegible-mono-latin-wght-normal.woff2';
import nextLatinExt from '../../../design/build/ts/fonts/atkinson-hyperlegible-next-latin-ext-wght-normal.woff2';
import nextLatin from '../../../design/build/ts/fonts/atkinson-hyperlegible-next-latin-wght-normal.woff2';
import { fonts } from '../../../design/build/ts/tokens.gen';

/** Unicode ranges of the two subsets, as in design/build/preview.html. */
const LATIN =
  'U+0000-00FF,U+0131,U+0152-0153,U+02BB-02BC,U+02C6,U+02DA,U+02DC,U+0304,U+0308,U+0329,U+2000-206F,U+20AC,U+2122,U+2191,U+2193,U+2212,U+2215,U+FEFF,U+FFFD';
const LATIN_EXT =
  'U+0100-02BA,U+02BD-02C5,U+02C7-02CC,U+02CE-02D7,U+02DD-02FF,U+0304,U+0308,U+0329,U+1D00-1DBF,U+1E00-1E9F,U+1EF2-1EFF,U+2020,U+20A0-20AB,U+20AD-20C0,U+2113,U+2C60-2C7F,U+A720-A7FF';

function face(family: string, url: string, range: string): string {
  return `@font-face { font-family: '${family}'; font-style: normal; font-display: swap; font-weight: 200 800; src: url(${JSON.stringify(url)}) format('woff2'); unicode-range: ${range}; }`;
}

/**
 * The CSS with the four font faces.
 *
 * @returns the @font-face rules of the sans and mono family, latin and latin-ext subsets
 */
export function fontFaceCss(): string {
  const [sans] = fonts.sans;
  const [mono] = fonts.mono;
  return [
    face(sans, nextLatin as string, LATIN),
    face(sans, nextLatinExt as string, LATIN_EXT),
    face(mono, monoLatin as string, LATIN),
    face(mono, monoLatinExt as string, LATIN_EXT),
  ].join('\n');
}

/**
 * Adds a style element with the font faces to the document head, once.
 *
 * @param doc - the document (the page Grafana runs in)
 */
export function ensureFonts(doc: Document = document): void {
  if (doc.head.querySelector('style[data-bothy-live-fonts]')) {
    return;
  }
  const style = doc.createElement('style');
  style.setAttribute('data-bothy-live-fonts', '');
  style.textContent = fontFaceCss();
  doc.head.appendChild(style);
}
