### Changed

- The defaults of `ratelimit.rl.upload` are 300 requests per minute with a burst of 60 (were 30
  and 10), so the uploads of two messages with `message.max_attachments` files fit the burst
  (05 section 10, addendum of 2026-10-30). Instances that set their own values keep them.
