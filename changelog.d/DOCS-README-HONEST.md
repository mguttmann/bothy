### Changed

- The README says what is true today: a status block at the top (in development, in daily use at
  one self-hosted installation, no release yet), the idea behind Bothy, how it is used today, what
  works on `main` with a link to its documentation, the known limits, the next steps of the
  roadmap and how to try it from source with `make image && make demo` and the app builds. The
  one-command quick start stays, marked as working from the first release on. The docs landing
  page and the roadmap say the same; the roadmap shows M0 done, M1 to M8 built and V1 in
  progress (DOCS-README-HONEST).
- The gate `install-options` also checks the release status: while `VERSION` is `0.0.0`,
  `README.md` and `docs/index.md` must say "No release yet", and from the first release on
  neither may (DOCS-README-HONEST).
