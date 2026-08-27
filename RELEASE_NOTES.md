# Changes since the last release

## New & Improved

- Table cells can now hold the keyboard focus (issue #45). Clicking a `Field` (or any other focusable widget) returned
  from `ColumnCell()` now edits it in place: the table keeps that cell installed while it has the focus, so keys,
  redraws, caret blinking and scroll-into-view all reach it. Tab and Shift-Tab move between the focusable cells of the
  table, and Return, Enter or Escape hand the focus back to the table. New `Table.FocusCell(row, col)` and
  `Table.FocusedCell()` methods allow editing to be started and inspected programmatically. The `focused` argument
  passed to `ColumnCell()` is now also true while one of the table's cells has the focus. Rows must return the same
  widget instance on every `ColumnCell()` call for this to work, and that instance must not be shared between rows or
  tables, so `CloneForTarget()` must not copy it into the clone. While a cell has the focus, it is handed the row's
  plain inks rather than the selection inks, so that it stands out from a selected row as the cell being edited and a
  `Field`'s own text selection stays visible within it; the row is still reported as selected.

## Bug Fixes

- (none yet)
