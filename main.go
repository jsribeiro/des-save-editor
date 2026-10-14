// Command des-save-editor is a small GUI editor for Demon's Souls (2020) USR-DATA saves.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/jsribeiro/des-save-editor/internal/save"
)

const (
	appTitle = "Demon's Souls Save Editor"
	// Assumed in-game name limit; the save format itself allows any length.
	maxNameLen = 16
	// Soul level is the attribute total minus 80 (confirmed in game at levels 10 and 79).
	soulLevelOffset = 80
	// In-game attribute range; 8 attributes at 99 give the maximum soul level, 712.
	minAttribute, maxAttribute = 1, 99
)

// field binds an entry to a top-level record by name.
type field struct {
	name     string
	entry    *widget.Entry
	rec      *save.Record
	onChange func()
}

type attrRow struct {
	base, cur *field
	bonus     int64 // CURRENT - BASE at the time sync was enabled (equipment bonuses)
}

type editor struct {
	win     fyne.Window
	path    string
	save    *save.Save
	dirty   bool
	loading bool

	fields    []*field
	attrs     []*attrRow
	total     *field
	sync      *widget.Check
	soulLevel *widget.Label
	playtime  *widget.Label
	status    *widget.Label

	souls, cumulative *field
	soulsSync         *widget.Check
	// Values when souls sync was last enabled; cumulative only grows with souls gained since then.
	soulsBaseline, cumulativeBaseline int64

	rawRows     []rawRow
	rawFiltered []int
	rawList     *widget.List
	rawFilter   *widget.Entry
	rawSelected int
	rawLabel    *widget.Label
	rawValue    *widget.Entry
	rawApply    *widget.Button

	saveBtn, saveAsBtn *widget.Button
	tabs               *container.AppTabs
}

type rawRow struct {
	rec   *save.Record
	depth int
}

func main() {
	a := app.NewWithID("io.github.jsribeiro.des-save-editor")
	ed := newEditor(a.NewWindow(appTitle))
	ed.win.Resize(fyne.NewSize(820, 760))
	if len(os.Args) > 1 {
		ed.openPath(os.Args[1])
	}
	ed.win.ShowAndRun()
}

func newEditor(win fyne.Window) *editor {
	ed := &editor{win: win, rawSelected: -1}
	win.SetContent(ed.build())
	win.SetOnDropped(func(_ fyne.Position, uris []fyne.URI) {
		if len(uris) > 0 {
			ed.confirmDiscard(func() { ed.openPath(uris[0].Path()) })
		}
	})
	win.SetCloseIntercept(func() { ed.confirmDiscard(win.Close) })
	ed.setLoaded(false)
	ed.loadForm()
	return ed
}

func (ed *editor) newField(name string) *field {
	f := &field{name: name, entry: widget.NewEntry()}
	f.entry.Validator = func(s string) error {
		if f.rec == nil {
			return nil
		}
		return f.rec.Check(s)
	}
	f.entry.OnChanged = func(s string) {
		if ed.loading || f.rec == nil || f.entry.Validator(s) != nil || f.rec.String() == s {
			return
		}
		if f.rec.Set(s) == nil {
			ed.markDirty()
			if f.onChange != nil {
				f.onChange()
			}
		}
	}
	ed.fields = append(ed.fields, f)
	return f
}

// limit adds a range check to the field's validator.
func (f *field) limit(lo, hi int64) {
	check := f.entry.Validator
	f.entry.Validator = func(s string) error {
		if err := check(s); err != nil || f.rec == nil {
			return err
		}
		if v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err != nil || v < lo || v > hi {
			return fmt.Errorf("must be %d–%d", lo, hi)
		}
		return nil
	}
}

func (ed *editor) build() fyne.CanvasObject {
	openBtn := widget.NewButtonWithIcon("Open…", theme.FolderOpenIcon(), func() { ed.confirmDiscard(ed.showOpen) })
	ed.saveBtn = widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), ed.saveInPlace)
	ed.saveAsBtn = widget.NewButtonWithIcon("Save As…", theme.DocumentSaveIcon(), ed.showSaveAs)
	ed.status = widget.NewLabel("Open a USR-DATA file, or drop one on this window.")
	ed.status.Truncation = fyne.TextTruncateEllipsis
	toolbar := container.NewBorder(nil, nil, container.NewHBox(openBtn, ed.saveBtn, ed.saveAsBtn), nil, ed.status)

	ed.tabs = container.NewAppTabs(
		container.NewTabItem("Character", container.NewVScroll(ed.buildCharacter())),
		container.NewTabItem("All fields", ed.buildRaw()),
	)
	ed.tabs.OnSelected = func(*container.TabItem) { ed.rawList.Refresh() }
	return container.NewBorder(toolbar, nil, nil, nil, ed.tabs)
}

func (ed *editor) buildCharacter() fyne.CanvasObject {
	name := ed.newField("PLAYER_NAME")
	name.entry.Validator = func(s string) error {
		if name.rec == nil {
			return nil
		}
		if n := len([]rune(s)); n == 0 || n > maxNameLen {
			return fmt.Errorf("name must be 1–%d characters", maxNameLen)
		}
		return nil
	}
	ed.souls = ed.newField("PLAYER_SOULS")
	ed.cumulative = ed.newField("PLAYER_CUMULATIVESOULS")
	ed.souls.onChange = ed.updateCumulative
	ed.soulsSync = widget.NewCheck("Raise Total souls gained when Souls goes up", func(on bool) {
		if on {
			ed.captureSoulsBaseline()
		}
		ed.applySyncState()
	})
	ed.soulsSync.Checked = true
	ed.playtime = widget.NewLabel("")
	character := widget.NewForm(
		widget.NewFormItem("Name", name.entry),
		widget.NewFormItem("Souls", ed.souls.entry),
		widget.NewFormItem("Total souls gained", ed.cumulative.entry),
		widget.NewFormItem("", ed.soulsSync),
		widget.NewFormItem("Play time", ed.playtime),
	)

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
		grid = append(grid, widget.NewLabel(titleCase(a)), row.base.entry, row.cur.entry)
	}
	ed.total = ed.newField("PLAYER_TOTALADDPARAM")
	ed.total.limit(minAttribute*int64(len(save.Attributes)), maxAttribute*int64(len(save.Attributes)))
	ed.total.onChange = ed.updateSoulLevel
	ed.soulLevel = widget.NewLabel("")
	grid = append(grid, widget.NewLabel("Total"), ed.total.entry, ed.soulLevel)

	ed.sync = widget.NewCheck("Keep Current values and Total in sync with Base", func(on bool) {
		if on {
			ed.captureBonuses()
			ed.updateTotal()
		}
		ed.applySyncState()
	})
	ed.sync.Checked = true
	attrs := container.NewVBox(
		container.NewGridWithColumns(3, grid...),
		ed.sync,
		hint("Current = Base + equipment bonus. Total is the sum of Base values; soul level is Total − 80. Base values are 1–99, so the maximum soul level is 712."),
	)

	// HP / MP / Stamina: Pool | Base max | Current max | Current
	pools := []fyne.CanvasObject{bold(""), bold("Base max"), bold("Current max"), bold("Current")}
	for _, p := range [][2]string{{"HP", "HP"}, {"MP", "MP"}, {"Stamina", "SP"}} {
		pools = append(pools, widget.NewLabel(p[0]),
			ed.newField("PLAYER_BASEMAX"+p[1]).entry,
			ed.newField("PLAYER_CURRENTMAX"+p[1]).entry,
			ed.newField("PLAYER_CURRENT"+p[1]).entry)
	}
	poolBox := container.NewVBox(
		container.NewGridWithColumns(4, pools...),
		hint("Raw values; they are not recalculated from attributes. In Soul Form, Current max HP is reduced."),
	)

	return container.NewVBox(
		widget.NewCard("Character", "", character),
		widget.NewCard("Attributes", "", attrs),
		widget.NewCard("HP / MP / Stamina", "", poolBox),
	)
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
	ed.loadForm() // a raw edit may change a field shown on the Character tab
	ed.rawList.Refresh()
}

// loadForm copies record values into the Character tab without triggering sync logic.
func (ed *editor) loadForm() {
	ed.loading = true
	for _, f := range ed.fields {
		f.rec = nil
		if ed.save != nil {
			f.rec = ed.save.FindName(f.name)
		}
		if f.rec == nil {
			f.entry.SetText("")
			f.entry.Disable()
		} else {
			f.entry.SetText(f.rec.String())
			f.entry.Enable()
		}
	}
	ed.loading = false
	ed.captureBonuses()
	ed.captureSoulsBaseline()
	ed.applySyncState()
	ed.updateSoulLevel()
	ed.playtime.SetText("")
	if ed.save != nil {
		if r := ed.save.FindName("PLAYTIME"); r != nil {
			if ms, err := strconv.ParseFloat(r.String(), 64); err == nil {
				ed.playtime.SetText((time.Duration(ms) * time.Millisecond).Round(time.Second).String())
			}
		}
	}
}

func (ed *editor) captureBonuses() {
	for _, a := range ed.attrs {
		if a.base.rec != nil && a.cur.rec != nil {
			a.bonus = a.cur.rec.Int() - a.base.rec.Int()
		}
	}
}

func (ed *editor) captureSoulsBaseline() {
	if ed.souls.rec != nil && ed.cumulative.rec != nil {
		ed.soulsBaseline, ed.cumulativeBaseline = ed.souls.rec.Int(), ed.cumulative.rec.Int()
	}
}

// updateCumulative mirrors the game: gaining souls raises the total, spending them doesn't lower it.
func (ed *editor) updateCumulative() {
	if !ed.soulsSync.Checked || ed.souls.rec == nil || ed.cumulative.rec == nil {
		return
	}
	gained := max(ed.souls.rec.Int()-ed.soulsBaseline, 0)
	if ed.cumulative.rec.SetInt(ed.cumulativeBaseline+gained) == nil {
		ed.cumulative.entry.SetText(ed.cumulative.rec.String())
	}
}

func (ed *editor) applySyncState() {
	setEnabled := func(f *field, on bool) {
		if f.rec != nil && on {
			f.entry.Enable()
		} else {
			f.entry.Disable()
		}
	}
	for _, f := range append(ed.currentFields(), ed.total) {
		setEnabled(f, !ed.sync.Checked)
	}
	setEnabled(ed.cumulative, !ed.soulsSync.Checked)
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

func (ed *editor) markDirty() {
	if !ed.dirty {
		ed.dirty = true
		ed.updateTitle()
	}
}

func (ed *editor) updateTitle() {
	t := appTitle
	if ed.path != "" {
		t += " — " + filepath.Base(ed.path)
	}
	if ed.dirty {
		t += " *"
	}
	ed.win.SetTitle(t)
}

func (ed *editor) setLoaded(ok bool) {
	for _, b := range []*widget.Button{ed.saveBtn, ed.saveAsBtn} {
		if ok {
			b.Enable()
		} else {
			b.Disable()
		}
	}
}

func (ed *editor) confirmDiscard(then func()) {
	if !ed.dirty {
		then()
		return
	}
	dialog.ShowConfirm("Unsaved changes", "Discard your unsaved changes?", func(ok bool) {
		if ok {
			then()
		}
	}, ed.win)
}

func (ed *editor) showOpen() {
	d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, ed.win)
			return
		}
		if r == nil {
			return
		}
		r.Close()
		ed.openPath(r.URI().Path())
	}, ed.win)
	d.Resize(fyne.NewSize(760, 560))
	d.Show()
}

func (ed *editor) openPath(path string) {
	b, err := os.ReadFile(path)
	if err == nil {
		var s *save.Save
		if s, err = save.Parse(b); err == nil {
			ed.save, ed.path, ed.dirty = s, path, false
		}
	}
	if err != nil {
		dialog.ShowError(fmt.Errorf("could not open %s: %w", filepath.Base(path), err), ed.win)
		return
	}
	ed.rawRows = ed.rawRows[:0]
	ed.save.Walk(func(r *save.Record, depth int) { ed.rawRows = append(ed.rawRows, rawRow{r, depth}) })
	ed.rawSelected = -1
	ed.filterRaw()
	ed.loadForm()
	ed.setLoaded(true)
	ed.updateTitle()
	ed.status.SetText(fmt.Sprintf("%s — %d bytes, checksum OK", path, len(b)))
}

// validate returns the first invalid entry's error, switching to the tab that shows it.
func (ed *editor) validate() error {
	for _, f := range ed.fields {
		if f.rec != nil {
			if err := f.entry.Validator(f.entry.Text); err != nil {
				ed.tabs.SelectIndex(0)
				return fmt.Errorf("%s: %w", f.name, err)
			}
		}
	}
	return nil
}

// encode serializes the save and re-parses it as a sanity check.
func (ed *editor) encode() ([]byte, error) {
	if err := ed.validate(); err != nil {
		return nil, err
	}
	out := ed.save.Bytes()
	if _, err := save.Parse(out); err != nil {
		return nil, fmt.Errorf("internal error, the edited save does not re-parse: %w", err)
	}
	return out, nil
}

func (ed *editor) saveInPlace() {
	out, err := ed.encode()
	if err != nil {
		dialog.ShowError(err, ed.win)
		return
	}
	backup := fmt.Sprintf("%s.%s.bak", ed.path, time.Now().Format("20060102-150405"))
	if err := copyFile(ed.path, backup); err != nil {
		dialog.ShowError(fmt.Errorf("could not write backup, nothing was saved: %w", err), ed.win)
		return
	}
	tmp := ed.path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		dialog.ShowError(err, ed.win)
		return
	}
	if err := os.Rename(tmp, ed.path); err != nil {
		dialog.ShowError(err, ed.win)
		return
	}
	ed.saved(fmt.Sprintf("Saved %s (backup: %s)", ed.path, filepath.Base(backup)))
}

func (ed *editor) showSaveAs() {
	out, err := ed.encode()
	if err != nil {
		dialog.ShowError(err, ed.win)
		return
	}
	d := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, ed.win)
			return
		}
		if w == nil {
			return
		}
		_, werr := w.Write(out)
		if cerr := w.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			dialog.ShowError(werr, ed.win)
			return
		}
		ed.path = w.URI().Path()
		ed.saved("Saved " + ed.path)
	}, ed.win)
	d.SetFileName(filepath.Base(ed.path))
	d.Resize(fyne.NewSize(760, 560))
	d.Show()
}

func (ed *editor) saved(msg string) {
	ed.dirty = false
	ed.updateTitle()
	ed.status.SetText(msg)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func bold(s string) fyne.CanvasObject {
	return widget.NewLabelWithStyle(s, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
}

func hint(s string) fyne.CanvasObject {
	l := widget.NewLabel(s)
	l.Wrapping = fyne.TextWrapWord
	l.Importance = widget.LowImportance
	return l
}

func titleCase(s string) string {
	return s[:1] + strings.ToLower(s[1:])
}
