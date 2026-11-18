### Fixed

- Background mode: a subagent of the Agent tool and a workflow are steps of the run again, so
  the live topology shows the agents of a background turn under their call, as in print mode.
  A background agent stays running until its notification, which ends it with its status and
  usage; `/metrics/subagents` counts the agents. Before, the runner mapped no task of a
  background turn, and the topology showed the run without its agents.
