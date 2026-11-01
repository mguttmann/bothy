### Added

- `StatsLiveRun` and `StatsTimelineRun` carry `chat` (`StatsRunChat`: thread, channel name,
  thread title) when the caller may read the thread (`thread.read`), in the gateway event
  `stats.live`, in `getStatsLive` and in `getStatsTimeline`; without that right the member is
  absent (25 4 rule 6).
