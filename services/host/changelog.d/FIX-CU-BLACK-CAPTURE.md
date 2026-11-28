### Fixed

- The Windows computer helper no longer sends an all-black screenshot of a still desktop. It
  opens a new DXGI Desktop Duplication for every capture, and the first frame of a duplication can
  be a pointer-only update (`LastPresentTime` 0) whose texture holds no desktop image. The helper
  now releases such frames and takes the next one with an image (at most 8, each wait bounded as
  before); when none comes it captures through GDI.
