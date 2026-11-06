### Changed

- CI: a pull request that changes only documentation no component reads (decisions, briefs,
  design notes, `mkdocs.yml`) runs no component test, platform, image or end-to-end job; the
  docs gates and the tools tests run as on every change. Documentation that a component's tests
  or generators read, such as the specification chapters, now runs that component's tests.
