# Changes since the last release

## Enhancements

- Added `Image.Preload()`, which decodes an image's pixels on the calling goroutine and caches them, so that the first
  draw only has to upload them. Images created from encoded data (`NewImageFromBytes()`, `NewImageFromFilePathOrURL()`)
  parse only their header up front and defer the pixel decode to the first draw, which happens on the UI thread and can
  stall it for seconds on a large image. `Preload()` may be called from any goroutine, and concurrently, so an
  application can decode its images on background goroutines instead.

## Bug Fixes

- (none yet)
