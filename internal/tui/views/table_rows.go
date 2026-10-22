package views

import "github.com/charmbracelet/bubbles/table"

// setTableRows replaces a table's rows and keeps its cursor usable.
//
// bubbles/table moves the cursor to -1 when it is handed an empty row set
// (SetRows: `if m.cursor > len(m.rows)-1 { m.cursor = len(m.rows) - 1 }`) and
// leaves it there when rows come back, because -1 is not greater than the new
// upper bound. An empty refresh is not unusual: config.yaml is read on every
// tick, and while a deployment rewrites it the read can see a truncated file.
// Without this reset, the next key press indexes names[-1] and panics with
// "index out of range [-1]".
func setTableRows(t *table.Model, rows []table.Row) {
	t.SetRows(rows)

	if len(rows) == 0 {
		return // SetCursor would clamp back to -1; nothing to select anyway.
	}
	if t.Cursor() < 0 {
		t.SetCursor(0)
		return
	}
	if t.Cursor() >= len(rows) {
		t.SetCursor(len(rows) - 1)
	}
}
