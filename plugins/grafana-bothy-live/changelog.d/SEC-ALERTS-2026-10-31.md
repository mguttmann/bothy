### Security

- react-router 6.30.6 under `react-router-dom-v5-compat` (a dependency of `@grafana/ui`) is
  overridden to 7.18.5, which fixes GHSA-337j-9hxr-rhxg and GHSA-wrjc-x8rr-h8h6; the advisory
  gate no longer ignores them. The plugin bundle is unchanged: react-router stays a webpack
  external that Grafana provides at runtime.
