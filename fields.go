package main

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2/widget"

	"github.com/jsribeiro/des-save-editor/internal/save"
)

// field binds an entry to a top-level record by name.
type field struct {
	name     string
	entry    *widget.Entry
	rec      *save.Record
	tab      int // index of the tab that shows the field
	onChange func()
}

// choice binds a drop-down to a top-level record whose values have known meanings.
type choice struct {
	key    uint32
	name   string // field name, or the key in hex if unknown
	sel    *widget.Select
	rec    *save.Record
	tab    int
	labels []string
	values []string // record values, parallel to labels
	// Record values for the options currently shown; a value not in values gets an extra "Unknown" option.
	shown    []string
	mirrors  []mirror
	onChange func()
}

// mirror is another record that stores the same choice, possibly encoded differently.
type mirror struct {
	path   []uint32
	values []string // parallel to the choice's values
}

// mirror adds a record that is kept in step with the choice; values are parallel to the choice's values.
func (c *choice) mirror(values []string, path ...uint32) *choice {
	c.mirrors = append(c.mirrors, mirror{path, values})
	return c
}

// index returns the position of the record's value in c.values, or -1.
func (c *choice) index() int {
	for i, v := range c.values {
		if c.rec != nil && v == c.rec.String() {
			return i
		}
	}
	return -1
}

func (ed *editor) newField(name string) *field {
	f := &field{name: name, entry: widget.NewEntry(), tab: ed.buildingTab}
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

func (ed *editor) newChoice(key uint32, labels, values []string) *choice {
	c := &choice{key: key, name: save.Label(key), tab: ed.buildingTab, labels: labels, values: values}
	c.sel = widget.NewSelect(nil, func(label string) {
		if c.onChange != nil {
			defer c.onChange()
		}
		if ed.loading || c.rec == nil {
			return
		}
		for i, l := range c.sel.Options {
			if l != label || c.rec.String() == c.shown[i] || c.rec.Set(c.shown[i]) != nil {
				continue
			}
			ed.markDirty()
			if j := c.index(); j >= 0 {
				for _, m := range c.mirrors {
					if r := ed.save.FindPath(m.path...); r != nil {
						r.Set(m.values[j])
					}
				}
			}
		}
	})
	ed.choices = append(ed.choices, c)
	return c
}

// load binds the choice to its record in s (nil for no save) and shows the record's value.
func (c *choice) load(s *save.Save) {
	c.rec = nil
	if s != nil {
		c.rec = s.Find(c.key)
	}
	if c.rec == nil {
		c.sel.Options, c.shown = nil, nil
		c.sel.ClearSelected()
		c.sel.Disable()
		return
	}
	opts, shown := append([]string{}, c.labels...), append([]string{}, c.values...)
	selected := c.index()
	if selected < 0 {
		opts, shown = append(opts, fmt.Sprintf("Unknown (%s)", c.rec.String())), append(shown, c.rec.String())
		selected = len(opts) - 1
	}
	c.sel.Options, c.shown = opts, shown
	c.sel.SetSelectedIndex(selected)
	c.sel.Enable()
}
