### Added

- "Expiring soon" in the administration offers "Extend" for a group membership: a new last day
  after the current one, on the same membership (`extendGroupMember`).
- The search popup of the app says when the search was capped and older sessions may be missing
  (`candidates_capped`).
- The generated Dart client knows `extendGroupMember`, `GroupMemberExtend` and
  `SearchAssistResult.candidates_capped`.

### Changed

- Adding or removing an owner and revoking a device need a typed reason in the administration;
  the other changes keep the default reason for an empty field.
- After an expired session the app deletes the local data but keeps the device keys and the
  device ID, so the next login is the same device and needs no new approval. Revocation, logout
  and a lost key still delete everything.
- A terminal tab on a device without a step-up key (Windows) shows a hint that names the web
  console and the other apps instead of "Something went wrong (open_failed)".

### Removed

- The feature switch "Title suggestions" (`supports_title`) of an agent.
