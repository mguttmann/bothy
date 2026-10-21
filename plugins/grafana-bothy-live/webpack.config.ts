// SPDX-License-Identifier: Apache-2.0
//
// Extends the webpack configuration of the scaffold (.config/webpack/webpack.config.ts, kept as
// generated; extension path of .config/README.md). Additions for the bundled fonts of the design
// tokens (docs/spec/25-visualization.md section 10 item 4): the font files land in dist/fonts/
// in every mode, and their OFL licence texts are copied next to them, as the OFL requires.

import CopyWebpackPlugin from 'copy-webpack-plugin';
import path from 'path';
import type { Configuration, RuleSetRule } from 'webpack';
import { merge } from 'webpack-merge';

import grafanaConfig, { type Env } from './.config/webpack/webpack.config.ts';

const FONTS = path.resolve(process.cwd(), '..', '..', 'design', 'build', 'ts', 'fonts');

const config = async (env: Env): Promise<Configuration> => {
  const base = await grafanaConfig(env);
  for (const rule of base.module?.rules ?? []) {
    const r = rule as RuleSetRule;
    if (r.test instanceof RegExp && r.test.test('x.woff2')) {
      r.generator = { filename: 'fonts/[name][ext]' };
    }
  }
  return merge(base, {
    plugins: [
      new CopyWebpackPlugin({
        patterns: [{ from: '*.OFL.txt', context: FONTS, to: 'fonts/[name][ext]' }],
      }),
    ],
  });
};

export default config;
