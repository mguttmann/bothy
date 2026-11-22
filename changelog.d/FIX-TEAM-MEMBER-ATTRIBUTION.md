### Fixed

- A member's mirrored reply in a team channel says "from @handle" with the one member it came
  from, also when the same agent is a member twice. The local database stores the new
  `team_handle` of a message (schema version 17).
