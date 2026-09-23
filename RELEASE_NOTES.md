# Changes since the last release

## New & Improved

- The accessible name a control takes from the label laid out before it is now read the way that label names itself,
  so a label whose `Accessibility.Name` differs from the text it draws — one drawn in small caps, say — names its
  neighbor in the same words. A custom panel that declares `role.Label` and an `Accessibility.Name` now serves as such
  a label too, a popup menu titles its menu after its `Accessibility.LabeledBy` label, and an icon-only table column
  header is named by its tooltip.

## Bug Fixes

-