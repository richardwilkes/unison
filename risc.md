# Risk of creating native objects during drawing

This document describes which patterns are safe and which patterns may lead to memory growth
or at least very high pressure on the GC and native resource release queues.

## What this is about

In this project, some objects wrap native Skia resources. If such objects are created inside
the `Draw()` path or in any logic that runs for every frame, memory can grow faster
than resources are released.

The most suspicious types are:
- `Paint`
- `Path`
- `Shader`
- `TextBlob`
- `ColorFilter`
- `MaskFilter`
- `ImageFilter`
- `PathEffect`

Not every one of these types is always a problem. The issue starts when objects are created
frequently, especially during animation, hover handling, clicking, or continuous full-window redraw.

## Good practice

### 1. Create once, use many times

If a filter or effect has a fixed configuration, it should be created once and only reused afterward.

Example:

```go
var disabledFilter = unison.Grayscale30Filter()
var dash = unison.DashEffect()

func draw(gc *unison.Canvas, r geom.Rect, paint *unison.Paint) {
    paint.SetColorFilter(disabledFilter)
    paint.SetPathEffect(dash)
    gc.DrawRect(r, paint)
}
```

Why this is good:
- the filter and effect are not recreated on every redraw,
- it avoids a flood of short-lived native objects,
- rendering cost stays more stable.

### 2. Create objects during view initialization, not inside `Draw`

If an object belongs to a specific control and its configuration does not change every frame,
it is best to build it once when the widget is created.

Example:

```go
type myWidget struct {
    blur *unison.MaskFilter
}

func newMyWidget() *myWidget {
    return &myWidget{
        blur: unison.NewBlurMaskFilter(blur.Normal, 2, true),
    }
}
```

Why this is good:
- the object is created once,
- it does not burden every following frame,
- its lifecycle is easier to control.

## Bad practice

### 1. Creating filters and effects on every redraw

Example:

```go
func draw(gc *unison.Canvas, r geom.Rect, paint *unison.Paint) {
    paint.SetColorFilter(unison.NewAlphaFilter(0.3))
    paint.SetPathEffect(unison.NewDashPathEffect([]float32{4, 4}, 0))
    gc.DrawRect(r, paint)
}
```

Why this is bad:
- every redraw creates new native objects,
- memory may grow under frequent redraw,
- even if this is not a permanent leak, it will look like one.

### 2. Creating objects inside animation or frame-by-frame code

Example:

```go
func draw(gc *unison.Canvas, r geom.Rect) {
    blur := unison.NewBlurMaskFilter(blur.Normal, 2, true)
    paint := unison.Red.Paint(gc, r, paintstyle.Fill)
    paint.SetMaskFilter(blur)
    gc.DrawRect(r, paint)
}
```

Why this is bad:
- animation or a timer will execute this code repeatedly,
- the number of allocations grows linearly with the number of frames,
- this pattern is very easy to mistake for a real memory leak.

## How to think about the risk

The simplest rule is:

- if something can be computed or created once, do it once,
- if something is used in `Draw()`, it should either be very lightweight or explicitly controlled,
- do not create native Skia wrappers in a hot path without a clear reason.

## What currently looks safe

In the current codebase, helpers such as:
- `Grayscale30Filter()`
- `Alpha30Filter()`
- `DashEffect()`

are cached, so by themselves they are a good usage pattern.

## What is still worth checking

If memory is still growing slowly, check first:
- whether some `Draw()` path still creates new `ColorFilter`, `MaskFilter`, `ImageFilter`,
  or `PathEffect`,
- whether SVG-related objects are recreated during redraw instead of during parsing.
