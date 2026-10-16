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
	// Width of the number entries in the Stats grids.
	valueWidth = 110
)

type editor struct {
	win     fyne.Window
	path    string
	save    *save.Save
	dirty   bool
	loading bool

	fields      []*field
	choices     []*choice
	buildingTab int // tab index given to fields as they are created

	attrs        []*attrRow
	total        *field
	sync         *widget.Check
	soulLevel    *widget.Label
	playtime     *widget.Label
	demonsKilled *widget.Label
	status       *widget.Label

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

func main() {
	a := app.NewWithID("io.github.jsribeiro.des-save-editor")
	a.Settings().SetTheme(newAppTheme(false))
	ed := newEditor(a.NewWindow(appTitle))
	// Tall enough to show the Stats tab, the tallest, without scrolling.
	ed.win.Resize(fyne.NewSize(880, 940))
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

func (ed *editor) build() fyne.CanvasObject {
	openBtn := widget.NewButtonWithIcon("Open…", theme.FolderOpenIcon(), func() { ed.confirmDiscard(ed.showOpen) })
	ed.saveBtn = widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), ed.saveInPlace)
	ed.saveAsBtn = widget.NewButtonWithIcon("Save As…", theme.DocumentSaveIcon(), ed.showSaveAs)
	ed.status = widget.NewLabel("Open a USR-DATA file, or drop one on this window.")
	ed.status.Truncation = fyne.TextTruncateEllipsis
	toolbar := container.NewBorder(nil, nil, container.NewHBox(openBtn, ed.saveBtn, ed.saveAsBtn), nil, ed.status)

	ed.tabs = container.NewAppTabs()
	for _, t := range []struct {
		name  string
		build func() fyne.CanvasObject
	}{
		{"Character", ed.buildCharacter},
		{"Stats", ed.buildStats},
	} {
		ed.buildingTab = len(ed.tabs.Items)
		// Each tab gets its own override: one around the tabs would miss the tabs not yet shown.
		ed.tabs.Append(container.NewTabItem(t.name, container.NewThemeOverride(container.NewVScroll(t.build()), newAppTheme(true))))
	}
	ed.tabs.Append(container.NewTabItem("All fields", ed.buildRaw()))
	ed.tabs.OnSelected = func(*container.TabItem) { ed.rawList.Refresh() }
	return container.NewBorder(toolbar, nil, nil, nil, ed.tabs)
}

// loadForm copies record values into the editing tabs without triggering sync logic.
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
	for _, c := range ed.choices {
		c.load(ed.save)
	}
	ed.loading = false
	ed.captureBonuses()
	ed.captureSoulsBaseline()
	ed.applySyncState()
	ed.updateSoulLevel()
	ed.playtime.SetText("")
	ed.demonsKilled.SetText("")
	if ed.save != nil {
		if r := ed.save.FindName("PLAYTIME"); r != nil {
			if ms, err := strconv.ParseFloat(r.String(), 64); err == nil {
				ed.playtime.SetText((time.Duration(ms) * time.Millisecond).Round(time.Second).String())
			}
		}
		if r := ed.save.FindName("DEMONSKILLED"); r != nil {
			ed.demonsKilled.SetText(r.String())
		}
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
				ed.tabs.SelectIndex(f.tab)
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

// hint is wrapped help text in a muted but readable colour.
func hint(s string) *widget.RichText {
	style := widget.RichTextStyleParagraph
	style.ColorName = colorNameHint
	t := widget.NewRichText(&widget.TextSegment{Text: s, Style: style})
	t.Wrapping = fyne.TextWrapWord
	return t
}

func setHint(t *widget.RichText, s string) {
	t.Segments[0].(*widget.TextSegment).Text = s
	t.Refresh()
}

// fixedWidth gives a value entry a set width, so grids of short numbers don't stretch across the window.
func fixedWidth(o fyne.CanvasObject) fyne.CanvasObject {
	return container.NewGridWrap(fyne.NewSize(valueWidth, o.MinSize().Height), o)
}

func titleCase(s string) string {
	return s[:1] + strings.ToLower(s[1:])
}
