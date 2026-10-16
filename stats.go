package main

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/jsribeiro/des-save-editor/internal/save"
)

type attrRow struct {
	base, cur *field
	bonus     int64 // CURRENT - BASE at the time sync was enabled (equipment bonuses)
}

func (ed *editor) buildStats() fyne.CanvasObject {
	// Attributes: Stat | Base | Current
	grid := []fyne.CanvasObject{bold("Attribute"), bold("Base"), bold("Current")}
	for _, a := range save.Attributes {
		row := &attrRow{base: ed.newField("PLAYER_BASE" + a), cur: ed.newField("PLAYER_CURRENT" + a)}
		// Current includes equipment bonuses, so only Base is limited.
		row.base.limit(minAttribute, maxAttribute)
		row.base.onChange = func() {
			if ed.sync.Checked {
				// Set the record directly; SetText alone would not apply an invalid value.
				if row.cur.rec != nil && row.cur.rec.SetInt(row.base.rec.Int()+row.bonus) == nil {
					row.cur.entry.SetText(row.cur.rec.String())
				}
				ed.updateTotal()
			}
		}
		ed.attrs = append(ed.attrs, row)
		grid = append(grid, widget.NewLabel(titleCase(a)), fixedWidth(row.base.entry), fixedWidth(row.cur.entry))
	}
	ed.total = ed.newField("PLAYER_TOTALADDPARAM")
	ed.total.limit(minAttribute*int64(len(save.Attributes)), maxAttribute*int64(len(save.Attributes)))
	ed.total.onChange = ed.updateSoulLevel
	ed.soulLevel = widget.NewLabel("")
	grid = append(grid, widget.NewLabel("Total"), fixedWidth(ed.total.entry), ed.soulLevel)

	ed.sync = widget.NewCheck("Keep Current values and Total in sync with Base", func(on bool) {
		if on {
			ed.captureBonuses()
			ed.updateTotal()
		}
		ed.applySyncState()
	})
	ed.sync.Checked = true
	attrs := container.NewVBox(
		container.NewHBox(container.NewGridWithColumns(3, grid...)),
		ed.sync,
		hint("Current = Base + equipment bonus. Total is the sum of Base values; soul level is Total − 80. Base values are 1–99, so the maximum soul level is 712."),
	)

	// HP / MP / Stamina: Pool | Base max | Current max | Current
	pools := []fyne.CanvasObject{bold(""), bold("Base max"), bold("Current max"), bold("Current")}
	for _, p := range [][2]string{{"HP", "HP"}, {"MP", "MP"}, {"Stamina", "SP"}} {
		pools = append(pools, widget.NewLabel(p[0]),
			fixedWidth(ed.newField("PLAYER_BASEMAX"+p[1]).entry),
			fixedWidth(ed.newField("PLAYER_CURRENTMAX"+p[1]).entry),
			fixedWidth(ed.newField("PLAYER_CURRENT"+p[1]).entry))
	}
	poolBox := container.NewVBox(
		container.NewHBox(container.NewGridWithColumns(4, pools...)),
		hint("Raw values; they are not recalculated from attributes. In Soul Form, Current max HP is reduced."),
	)

	return container.NewVBox(
		widget.NewCard("Attributes", "", attrs),
		widget.NewCard("HP / MP / Stamina", "", poolBox),
	)
}

func (ed *editor) captureBonuses() {
	for _, a := range ed.attrs {
		if a.base.rec != nil && a.cur.rec != nil {
			a.bonus = a.cur.rec.Int() - a.base.rec.Int()
		}
	}
}

func (ed *editor) currentFields() []*field {
	fs := make([]*field, len(ed.attrs))
	for i, a := range ed.attrs {
		fs[i] = a.cur
	}
	return fs
}

func (ed *editor) updateTotal() {
	if ed.total.rec == nil {
		return
	}
	var sum int64
	for _, a := range ed.attrs {
		if a.base.rec != nil {
			sum += a.base.rec.Int()
		}
	}
	if ed.total.rec.SetInt(sum) == nil {
		ed.total.entry.SetText(ed.total.rec.String())
	}
	ed.updateSoulLevel()
}

func (ed *editor) updateSoulLevel() {
	if ed.total.rec == nil {
		ed.soulLevel.SetText("")
		return
	}
	ed.soulLevel.SetText(fmt.Sprintf("Soul level %d", ed.total.rec.Int()-soulLevelOffset))
}
