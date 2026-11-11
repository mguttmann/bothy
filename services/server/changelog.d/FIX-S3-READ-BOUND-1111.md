### Security

- The image processing reads an original only up to the stored size of its file. Since the S3
  driver reads objects of any size, an object storage that answered more than was stored, or an
  object that was replaced behind the server, could make the worker hold the whole answer in
  memory: for a WebP that stays unchanged, and for the colour profile of a PNG. Such an object is
  now a read error; the job fails, is retried and stores nothing.
- The S3 driver hands out the body of a ranged read only when the storage answers the range
  asked for (status `206` with a matching `Content-Range`). A storage that ignores the range and
  answers the whole object is a storage failure instead of other and more bytes than asked for.
  An S3-compatible storage, or a proxy in front of it, has to answer a range as S3 does.

### Fixed

- An error answer of the object storage above `egress.max_response_bytes` is a storage failure
  whatever its status. Before, such an answer with status 404 or 416 was reported as a missing
  object or an invalid range, although the error document was not read.
