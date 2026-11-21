### Fixed

- The apps no longer replace the key of their local database. When the secure storage of the
  device did not return the key of an existing database at one start, the app created a new key,
  and the local data of the instance could never be opened again ("The app cannot read its local
  data"). Now no new key is created while the database exists.
- A local database that the device cannot read at the moment, for example on a locked iPhone, is
  no longer reported as unreadable, and without a signed-in session it is no longer deleted:
  the app shows "Try again" and opens it once the device is unlocked.
