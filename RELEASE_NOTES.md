# Changes since the last release

## Enhancements

- Added `Image.Preload()`, which decodes an image's pixels on the calling goroutine and caches them, so that the first
  draw only has to upload them. Images created from encoded data (`NewImageFromBytes()`, `NewImageFromFilePathOrURL()`)
  parse only their header up front and defer the pixel decode to the first draw, which happens on the UI thread and can
  stall it for seconds on a large image. `Preload()` may be called from any goroutine, and concurrently, so an
  application can decode its images on background goroutines instead.
- Sped up the CPU-rendering presentation path. The Windows GDI repack and the X11 image upload now convert their pixels
  with the kernels in the new `internal/pixconv` package. Building with `GOEXPERIMENT=simd` opts into the vector forms
  of those conversions; the scalar forms remain the default and the two produce bit-identical output.
- Removed the per-frame allocation in the X11 CPU presentation path. The buffer each chunk is converted into for the
  wire is now held on the connection and reused, so a steady stream of same-sized frames uploads without allocating.
- Restructured the width computation for `Text`. The rune widths are now summed in a single pass, and each font is
  asked for its metrics once per decoration run rather than once per rune. Widths and positions may differ from the
  values previously computed by a last-ULP amount, since the sums accumulate in a different order.
- `NewImageFromDrawing()` no longer copies the pixels it just rendered a second time. It hands its private buffer to
  the new image directly instead of routing through `NewImageFromPixels()`, whose defensive copy of a caller's slice
  was pure overhead here.
- Removed the slice copies made when passing points and colors to the underlying canvas layer. `Canvas.DrawPoints()`,
  `Path.Poly()` and the gradient shader constructors now pass a view of the caller's slice instead of a freshly
  allocated conversion of it.

## Bug Fixes

- Fixed an out-of-bounds write on Windows when a cursor or title icon was created from a sub-image. The pixel copy into
  the icon's DIB section indexed the source from offset 0 and sized the destination by the length of the source's pixel
  slice, so an image with a non-zero origin or a stride wider than its rows copied its padding and wrote past the end
  of the DIB. The drag image's DIB view had the same sizing problem. Both copies now walk the source a row at a time.
