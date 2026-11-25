### Changed

- Tested against Hermes Agent main `b39dd91f689a` (new pin). Hermes' dotenv loader takes an
  optional answer whether a plugin runs the profile elsewhere; the adapter leaves the question to
  Hermes, which keeps loading an ordinary profile's `.env` as before.
