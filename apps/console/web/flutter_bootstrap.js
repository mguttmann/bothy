// SPDX-License-Identifier: Apache-2.0
//
// Loader configuration of the web console (brief M6-02 Design note 3, docs/DECISIONS.md
// D-M6-02-3). No serviceWorkerSettings: the Flutter service worker is never registered, so no
// stale copy of the console outlives an update of the server. CanvasKit comes from the build
// output (`flutter build web --no-web-resources-cdn`), never from a CDN. M6-03 extends this
// configuration (CSP nonce).
{{flutter_js}}
{{flutter_build_config}}

_flutter.loader.load({});
