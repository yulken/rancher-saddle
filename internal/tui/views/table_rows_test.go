package views

import (
	"testing"

	"github.com/charmbracelet/bubbles/table"
	"github.com/stretchr/testify/assert"
)

func newTestTable() table.Model {
	return table.New(table.WithColumns([]table.Column{{Title: "name", Width: 10}}))
}

func TestSetTableRows_CursorRecoversAfterEmptyRefresh(t *testing.T) {
	tb := newTestTable()

	// An empty refresh (e.g. config.yaml caught mid-write) parks the cursor at -1.
	setTableRows(&tb, nil)
	assert.Equal(t, -1, tb.Cursor())

	// Rows coming back must restore a valid cursor, otherwise names[-1] panics.
	setTableRows(&tb, []table.Row{{"a"}})
	assert.Equal(t, 0, tb.Cursor())
}

func TestSetTableRows_ClampsCursorOnShrink(t *testing.T) {
	tb := newTestTable()
	setTableRows(&tb, []table.Row{{"a"}, {"b"}, {"c"}})
	tb.SetCursor(2)

	setTableRows(&tb, []table.Row{{"a"}})
	assert.Equal(t, 0, tb.Cursor())
}

func TestSetTableRows_NoRowsFromStart(t *testing.T) {
	tb := newTestTable()
	setTableRows(&tb, nil)
	assert.Equal(t, -1, tb.Cursor())

	setTableRows(&tb, nil)
	assert.Equal(t, -1, tb.Cursor())
}
