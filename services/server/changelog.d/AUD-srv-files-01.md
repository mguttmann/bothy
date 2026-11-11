### Fixed

- The S3 driver of the object storage reads an object of any size again: the body of an object
  is as long as the storage announces and is no longer cut at `file.max_upload_bytes`, so an
  export package or a converted original larger than that value can be downloaded. Every other
  answer of the storage is at most `egress.max_response_bytes`, and an object answer without a
  length is a storage failure.

### Security

- The pixel guard of the image processing also bounds the memory the pixel count does not: an
  image with an edge longer than 65535 pixels gets no thumbnails, and a GIF counts a fixed
  overhead for every frame, so a GIF of many tiny frames above `file.image.max_pixels` stays
  unchanged instead of being decoded. Before, a PNG of one pixel by many millions or a GIF of
  many frames of one pixel could make the worker ask for gigabytes of memory.
