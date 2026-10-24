### Fixed

- Every `404 not_found` answer of the HTTP API has the same body (R-AZ-01): a detail, reason,
  hint, current value or extension member of a use case no longer reaches the answer. Before, for
  example, `DELETE /channels/{channel_id}/project-binding` on a forum without a binding answered
  with a detail that an unknown forum did not get.
