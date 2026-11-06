### Changed

- Rulings, specification notes, design notes, tests and fixtures describe reported problems
  neutrally, without dates, counts, timings or configuration of a particular installation, and
  test fixtures use synthetic file names, amounts and counts.
- The mobile end-to-end runner (`tools/e2e/mobile_e2e.py`) picks the host ports of fakepush and
  patrol only from the dynamic port range (49152 to 65535) instead of skipping a fixed list.
