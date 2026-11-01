### Fixed

- Costs of runs on a personal subscription are shown again in the statistics (costs per agent,
  per space and per billing mode), in the backend view and in the thread costs, labeled as
  information. `billing.show_personal_estimates` now defaults to `true`; set it to `false` to hide
  these amounts. Before, an instance whose agents all ran on personal subscriptions showed "No
  data" for every cost view.
