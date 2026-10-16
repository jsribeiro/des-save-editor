package main

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/jsribeiro/des-save-editor/internal/save"
)

type rawRow struct {
	rec   *save.Record
	depth int
}

func (ed *editor) buildRaw() fyne.CanvasObject {
	ed.rawFilter = widget.NewEntry()
	ed.rawFilter.SetPlaceHolder("Filter by name, key or value…")
	ed.rawFilter.OnChanged = func(string) { ed.filterRaw() }

	ed.rawList = widget.NewList(
		func() int { return len(ed.rawFiltered) },
		func() fyne.CanvasObject {
			l := widget.NewLabel("")
			l.TextStyle.Monospace = true
			l.Truncation = fyne.TextTruncateEllipsis
			return l
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(ed.rawText(ed.rawRows[ed.rawFiltered[i]]))
		},
	)
	ed.rawList.OnSelected = func(i widget.ListItemID) {
		ed.rawSelected = ed.rawFiltered[i]
		r := ed.rawRows[ed.rawSelected].rec
		ed.rawLabel.SetText(fmt.Sprintf("%s (%s)", save.Label(r.Key), r.TypeName()))
		ed.rawValue.SetText(r.String())
		if r.IsEditable() {
			ed.rawValue.Enable()
			ed.rawApply.Enable()
		} else {
			ed.rawValue.Disable()
			ed.rawApply.Disable()
		}
	}

	ed.rawLabel = widget.NewLabel("Select a field to edit it.")
	ed.rawValue = widget.NewEntry()
	ed.rawValue.Disable()
	ed.rawApply = widget.NewButton("Apply", ed.applyRaw)
	ed.rawApply.Disable()
	ed.rawValue.OnSubmitted = func(string) { ed.applyRaw() }
	editRow := container.NewVBox(ed.rawLabel, container.NewBorder(nil, nil, nil, ed.rawApply, ed.rawValue))

	return container.NewBorder(ed.rawFilter, editRow, nil, nil, ed.rawList)
}

func (ed *editor) rawText(row rawRow) string {
	r := row.rec
	return fmt.Sprintf("%06X  %s%-28s %-6s %s", r.Offset, strings.Repeat("  ", row.depth), save.Label(r.Key), r.TypeName(), r.String())
}

func (ed *editor) filterRaw() {
	q := strings.ToLower(strings.TrimSpace(ed.rawFilter.Text))
	ed.rawFiltered = ed.rawFiltered[:0]
	for i, row := range ed.rawRows {
		if q == "" || strings.Contains(strings.ToLower(ed.rawText(row)), q) {
			ed.rawFiltered = append(ed.rawFiltered, i)
		}
	}
	ed.rawList.UnselectAll()
	ed.rawList.Refresh()
}

func (ed *editor) applyRaw() {
	if ed.rawSelected < 0 {
		return
	}
	r := ed.rawRows[ed.rawSelected].rec
	if r.String() == ed.rawValue.Text {
		return
	}
	if err := r.Set(ed.rawValue.Text); err != nil {
		dialog.ShowError(err, ed.win)
		return
	}
	ed.markDirty()
	ed.loadForm() // a raw edit may change a field shown on another tab
	ed.rawList.Refresh()
}
