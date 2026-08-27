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
- New `Field.InstallAccessoryPanel(panel)` attaches a panel to the right end of a `Field`, inside its border. The
  editable text area shrinks by the accessory's preferred width, and the field's focus borders are adjusted to match.
- New `NewComboField(options, initial, changedCallback)` creates a `Field` with a dropdown button embedded at its right
  end. Clicking the button (or pressing the down arrow while the field has the focus) pops up a menu of the options,
  and the field also accepts free-form typing, so the value need not be one of the options. Options are `*string`: a
  `nil` option means "not set" and an empty string means "empty"; these are shown as the «not set» and «empty»
  watermarks rather than as text. Duplicate options (compared case-insensitively) are dropped. `changedCallback` is
  invoked only when the value actually changes, with the new value (`nil` for "not set"). Clearing the text yields
  `nil` when a `nil` option was provided, an empty string when an empty option was provided, and is otherwise treated
  as invalid and does not invoke the callback. The field's minimum width is sized to fit the widest option.

## Bug Fixes

- (none yet)
