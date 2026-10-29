### Fixed

- `bothy import discord` places the new spaces after the last space of the server and the new
  channels of an existing target space after its last channel, each as one block in Discord order;
  before, their sort keys started at `a0` and interleaved with what was there (22 5.1 rule 4).
