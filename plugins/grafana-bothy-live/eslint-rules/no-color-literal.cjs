// SPDX-License-Identifier: Apache-2.0
/**
 * ESLint rule bothy/no-color-literal: no colour literal in the plugin's source code.
 *
 * Colours come only from design/build/ts/tokens.gen.ts (docs/spec/25-visualisierung.md
 * section 10; brief M7-25 requirement 14). The rule reports a string or template literal that
 * holds a hex colour, a CSS colour function (rgb, rgba, hsl, hsla, hwb, lab, lch, oklab, oklch,
 * color), a named CSS colour as the whole value, or a named colour after a colour property in
 * CSS text (`color: navy`). The generated token file lies outside src/ and is not linted by it.
 *
 * Example: `{ color: 'red' }` and `'1px solid #136f69'` are reported; `{ color: theme.colors.text.primary }`
 * is not.
 */

'use strict';

/** CSS named colours (CSS Color Module Level 4) and `transparent`. */
const NAMED = new Set(
  (
    'aliceblue antiquewhite aqua aquamarine azure beige bisque black blanchedalmond blue blueviolet brown ' +
    'burlywood cadetblue chartreuse chocolate coral cornflowerblue cornsilk crimson cyan darkblue darkcyan ' +
    'darkgoldenrod darkgray darkgreen darkgrey darkkhaki darkmagenta darkolivegreen darkorange darkorchid ' +
    'darkred darksalmon darkseagreen darkslateblue darkslategray darkslategrey darkturquoise darkviolet ' +
    'deeppink deepskyblue dimgray dimgrey dodgerblue firebrick floralwhite forestgreen fuchsia gainsboro ' +
    'ghostwhite gold goldenrod gray green greenyellow grey honeydew hotpink indianred indigo ivory khaki ' +
    'lavender lavenderblush lawngreen lemonchiffon lightblue lightcoral lightcyan lightgoldenrodyellow ' +
    'lightgray lightgreen lightgrey lightpink lightsalmon lightseagreen lightskyblue lightslategray ' +
    'lightslategrey lightsteelblue lightyellow lime limegreen linen magenta maroon mediumaquamarine ' +
    'mediumblue mediumorchid mediumpurple mediumseagreen mediumslateblue mediumspringgreen ' +
    'mediumturquoise mediumvioletred midnightblue mintcream mistyrose moccasin navajowhite navy oldlace ' +
    'olive olivedrab orange orangered orchid palegoldenrod palegreen paleturquoise palevioletred ' +
    'papayawhip peachpuff peru pink plum powderblue purple rebeccapurple red rosybrown royalblue ' +
    'saddlebrown salmon sandybrown seagreen seashell sienna silver skyblue slateblue slategray slategrey ' +
    'snow springgreen steelblue tan teal thistle tomato turquoise violet wheat white whitesmoke yellow ' +
    'yellowgreen transparent'
  ).split(' ')
);

const HEX = /#(?:[0-9a-f]{3}|[0-9a-f]{4}|[0-9a-f]{6}|[0-9a-f]{8})\b/i;
const FUNCTION = /\b(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch|color)\(/i;
const DECLARATION =
  /(?:^|[;{\s])(?:color|background(?:-color)?|border(?:-[a-z]+)*|outline(?:-color)?|fill|stroke|text-decoration-color|caret-color|accent-color)\s*:\s*([^;]*)/gi;

/**
 * Whether a piece of literal text holds a colour.
 *
 * @param {string} text - the cooked text of a string or template literal
 * @returns {boolean} true when the text is or contains a colour literal
 */
function hasColor(text) {
  if (HEX.test(text) || FUNCTION.test(text)) {
    return true;
  }
  if (NAMED.has(text.trim().toLowerCase())) {
    return true;
  }
  for (const match of text.matchAll(DECLARATION)) {
    const words = (match[1] || '').toLowerCase().split(/[\s,()]+/);
    if (words.some((w) => NAMED.has(w))) {
      return true;
    }
  }
  return false;
}

/** @type {import('eslint').Rule.RuleModule} */
module.exports = {
  meta: {
    type: 'problem',
    docs: { description: 'Colours only from the design tokens (design/build/ts/tokens.gen.ts)' },
    messages: {
      literal: 'Colour literal "{{text}}": take colours only from the design tokens (design/build/ts/tokens.gen.ts).',
    },
    schema: [],
  },
  create(context) {
    const report = (node, text) => context.report({ node, messageId: 'literal', data: { text: text.slice(0, 40) } });
    return {
      Literal(node) {
        if (typeof node.value === 'string' && hasColor(node.value)) {
          report(node, node.value);
        }
      },
      TemplateLiteral(node) {
        const text = node.quasis.map((q) => q.value.cooked ?? q.value.raw).join(' ');
        if (hasColor(text)) {
          report(node, text);
        }
      },
    };
  },
  hasColor,
};
