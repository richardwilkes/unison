# Proposal: `FrameResourceTracker` as a shared per-frame resource layer

## Goal

Introduce a separate layer responsible for temporary resources that live for a single frame.

## Proposed

Instead of keeping many separate fields in `Canvas`, such as:
- `ownedPaint`
- `ownedPath`
- `ownedShader`
- `ownedText`

it is worth introducing a dedicated tracker for one frame, for example:

```go
type Disposable interface {
    Dispose()
}

type FrameResourceTracker struct {
    resources []Disposable
}

func (t *FrameResourceTracker) Track(d Disposable) {
    if d != nil {
        t.resources = append(t.resources, d)
    }
}

func (t *FrameResourceTracker) Release() {
    for _, r := range t.resources {
        r.Dispose()
    }
    clear(t.resources)
    t.resources = t.resources[:0]
}
```

Then in `Canvas`:

```go
type Canvas struct {
    canvas   skia.Canvas
    surface  *surface
    frameRes FrameResourceTracker
}
```

and after drawing completes:

```go
func (c *Canvas) Flush() {
    c.surface.flush(true)
    c.frameRes.Release()
}
```

## Why this is better than the current state

### 1. A single place of responsibility

Today the responsibility is spread across several lists and several separate
release methods. After this change there would be one place responsible for
temporary drawing resources.

That makes the code simpler and reduces the number of places that must be
remembered when adding a new wrapper around Skia.

### 2. Better scalability

If more temporary object types appear in the future, there will be no need to
add another field to `Canvas` and another `releaseOwnedXxx()` function.

It will be enough to:
- implement `Dispose()`
- register the object through the frame tracker

### 3. Lower risk of regression

The current model is effective, but it is easier to make mistakes in it. It is
easy to imagine a situation where new code creates another native object per
frame and the author forgets to add yet another list to `Canvas`.

A frame tracker reduces that risk.

### 4. Better semantics

`FrameResourceTracker` clearly describes what it manages:
- temporary resources for a single frame,
- not global resources,
- not caches,
- not reusable long-lived objects.

This is easier to understand than several separate collections inside
`Canvas`.

## Important rule: not everything should be tracked

This is the key point.

A frame tracker should not take ownership of everything that has `Dispose()`.
It should manage only temporary resources created for a single redraw.

### Good candidates for the tracker
- `Paint` created in `Color.Paint()`
- `Path` created temporarily inside `Draw()`
- `Shader` built dynamically for a single draw
- `TextBlob` created only for the current frame

### Bad candidates for the tracker
- a global cache such as `Grayscale30Filter()`
- a global `DashEffect()`
- theme-level objects shared between widgets
- resources that should outlive a single frame

So the tracker does not solve everything automatically. A conscious split is
still needed between:
- **temporary per-frame resources**
- **shared/cache resources**

## How this helps future theme and styling work

This solution matters not only for memory. It is also important for future
expansion of the styling system.

If the toolkit is expected to support more ambitious visual styles in the
future — for example a theme closer to FlatLaf — the renderer of many
components will become more complex.

Such a renderer usually creates more temporary objects during drawing, for
example:
- extra outlines,
- separate shadows,
- hover and focus layers,
- more complex paths,
- dynamic gradients and shaders,
- variants for disabled, pressed, and selected states.

Without a shared per-frame resource model, every such change increases the
risk of reintroducing memory growth.

With `FrameResourceTracker`, styling can evolve more aggressively because the
infrastructure already provides a safe place to register temporary drawing
resources.

In other words:
- better resource infrastructure means safer renderer expansion,
- safer renderers mean more freedom for more advanced themes.

This will not automatically make the theme system as flexible as FlatLaf, but
it removes one of the technical obstacles that currently limit more ambitious
styling.

