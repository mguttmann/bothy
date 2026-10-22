// SPDX-License-Identifier: Apache-2.0
//
// Loader configuration of the web console (briefs M6-02 Design note 3 and M6-03, docs/DECISIONS.md
// D-M6-02-3 and D-M6-03-2). No serviceWorkerSettings: the Flutter service worker is never
// registered, so no stale copy of the console outlives an update of the server. Every URL the
// loader and the engine use is derived from the URL of this script, because index.html has no
// <base> element (the CSP has base-uri 'none'): the entry point, the assets, CanvasKit
// (`flutter build web --no-web-resources-cdn`) and the fallback fonts all stay on the own origin
// (docs/spec/19-security.md 5 rule 1). The nonce of the script tag, which the server sets per
// response, goes to the loader option nonce for the styles and scripts the engine inserts (rule 2).
{{flutter_js}}
{{flutter_build_config}}

(function () {
  const script = document.currentScript;
  const base = new URL('.', script.src).href;
  _flutter.loader.load({
    config: {
      entrypointBaseUrl: base,
      assetBase: base,
      canvasKitBaseUrl: base + 'canvaskit/',
      fontFallbackBaseUrl: base + 'assets/fonts/fallback/',
      nonce: script.nonce,
    },
  });
})();
