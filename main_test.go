package main

import (
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/jsribeiro/des-save-editor/internal/save"
)

func openSample(t *testing.T) (*editor, string) {
	t.Helper()
	b, err := os.ReadFile("samples/USR-DATA-2")
	if err != nil {
		t.Skipf("sample not available: %v", err)
	}
	path := filepath.Join(t.TempDir(), "USR-DATA")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	test.NewTempApp(t)
	ed := newEditor(test.NewTempWindow(t, nil))
	ed.openPath(path)
	if ed.save == nil {
		t.Fatal("sample did not open")
	}
	return ed, path
}

func fieldByName(t *testing.T, ed *editor, name string) *field {
	t.Helper()
	for _, f := range ed.fields {
		if f.name == name {
			return f
		}
	}
	t.Fatalf("no field %s", name)
	return nil
}

func typeInto(f *field, s string) {
	f.entry.SetText("")
	test.Type(f.entry, s)
}

func reload(t *testing.T, path string) *save.Save {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s, err := save.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSyncedEditAndSave(t *testing.T) {
	ed, path := openSample(t)
	if got := fieldByName(t, ed, "PLAYER_NAME").entry.Text; got != "John Souls" {
		t.Fatalf("name = %q", got)
	}
	typeInto(fieldByName(t, ed, "PLAYER_BASESTRENGTH"), "20")
	typeInto(fieldByName(t, ed, "PLAYER_SOULS"), "99999")
	typeInto(fieldByName(t, ed, "PLAYER_NAME"), "Jane Souls")
	if got := ed.total.entry.Text; got != "94" {
		t.Errorf("total entry = %s, want 94", got)
	}
	if !ed.dirty {
		t.Error("editor not marked dirty")
	}
	ed.saveInPlace()

	s := reload(t, path)
	for name, want := range map[string]string{
		"PLAYER_NAME":            "Jane Souls",
		"PLAYER_SOULS":           "99999",
		"PLAYER_BASESTRENGTH":    "20",
		"PLAYER_CURRENTSTRENGTH": "20",
		"PLAYER_TOTALADDPARAM":   "94",
		"PLAYER_BASEDEXTERITY":   "10",
	} {
		if got := s.FindName(name).String(); got != want {
			t.Errorf("%s = %s, want %s", name, got, want)
		}
	}
	if backups, _ := filepath.Glob(path + ".*.bak"); len(backups) != 1 {
		t.Errorf("want 1 backup, got %v", backups)
	} else if _, err := save.Parse(mustRead(t, backups[0])); err != nil {
		t.Errorf("backup is not a valid save: %v", err)
	}
}

func TestUnsyncedEdit(t *testing.T) {
	ed, path := openSample(t)
	ed.sync.SetChecked(false)
	typeInto(fieldByName(t, ed, "PLAYER_BASELUCK"), "9")
	typeInto(fieldByName(t, ed, "PLAYER_CURRENTLUCK"), "12")
	ed.saveInPlace()

	s := reload(t, path)
	for name, want := range map[string]string{
		"PLAYER_BASELUCK":      "9",
		"PLAYER_CURRENTLUCK":   "12",
		"PLAYER_TOTALADDPARAM": "90", // untouched while sync is off
	} {
		if got := s.FindName(name).String(); got != want {
			t.Errorf("%s = %s, want %s", name, got, want)
		}
	}
}

func TestInvalidValueBlocksSave(t *testing.T) {
	ed, path := openSample(t)
	before := mustRead(t, path)
	typeInto(fieldByName(t, ed, "PLAYER_SOULS"), "-5")
	ed.saveInPlace()
	if string(mustRead(t, path)) != string(before) {
		t.Error("file changed despite invalid value")
	}
}

func TestSoulsSync(t *testing.T) {
	ed, path := openSample(t) // souls 4450, cumulative 9301
	cumulative := fieldByName(t, ed, "PLAYER_CUMULATIVESOULS")
	if !cumulative.entry.Disabled() {
		t.Error("cumulative should be read-only while synced")
	}
	souls := fieldByName(t, ed, "PLAYER_SOULS")
	typeInto(souls, "5000")
	if got := cumulative.entry.Text; got != "9851" {
		t.Errorf("after raising souls, cumulative = %s, want 9851", got)
	}
	typeInto(souls, "100")
	if got := cumulative.entry.Text; got != "9301" {
		t.Errorf("after lowering souls, cumulative = %s, want 9301", got)
	}
	typeInto(souls, "6000")
	ed.saveInPlace()
	s := reload(t, path)
	if got := s.FindName("PLAYER_CUMULATIVESOULS").String(); got != "10851" {
		t.Errorf("saved cumulative = %s, want 10851", got)
	}

	ed.soulsSync.SetChecked(false)
	if cumulative.entry.Disabled() {
		t.Error("cumulative should be editable when sync is off")
	}
	typeInto(souls, "7000")
	if got := cumulative.entry.Text; got != "10851" {
		t.Errorf("cumulative changed with sync off: %s", got)
	}
	// Re-enabling takes a new baseline from the current values.
	ed.soulsSync.SetChecked(true)
	typeInto(souls, "7500")
	if got := cumulative.entry.Text; got != "11351" {
		t.Errorf("after re-enabling sync, cumulative = %s, want 11351", got)
	}
}

func TestAttributeLimits(t *testing.T) {
	ed, path := openSample(t)
	str := fieldByName(t, ed, "PLAYER_BASESTRENGTH")
	typeInto(str, "99")
	if got := ed.total.entry.Text; got != "173" {
		t.Errorf("total = %s, want 173", got)
	}
	typeInto(str, "100")
	if str.entry.Validate() == nil || str.rec.String() == "100" {
		t.Errorf("100 accepted (record = %s)", str.rec.String())
	}
	before := mustRead(t, path)
	ed.saveInPlace()
	if string(mustRead(t, path)) != string(before) {
		t.Error("saved despite an out-of-range attribute")
	}
	typeInto(str, "0")
	if str.entry.Validate() == nil {
		t.Error("0 accepted")
	}

	// With sync off, Total is editable but still limited to 8–792.
	typeInto(str, "16")
	ed.sync.SetChecked(false)
	typeInto(ed.total, "793")
	if ed.total.entry.Validate() == nil || ed.total.rec.String() == "793" {
		t.Errorf("793 accepted (record = %s)", ed.total.rec.String())
	}
	typeInto(ed.total, "792")
	if ed.total.rec.String() != "792" || ed.soulLevel.Text != "Soul level 712" {
		t.Errorf("total = %s, label = %q", ed.total.rec.String(), ed.soulLevel.Text)
	}
}

func choiceByName(t *testing.T, ed *editor, name string) *choice {
	t.Helper()
	for _, c := range ed.choices {
		if c.name == name {
			return c
		}
	}
	t.Fatalf("no choice %s", name)
	return nil
}

func TestCharacterChoices(t *testing.T) {
	ed, path := openSample(t)
	class, body := choiceByName(t, ed, "PLAYER_ARCHETYPE"), choiceByName(t, ed, "PLAYER_ISMALE")
	if class.sel.Selected != "Knight" || body.sel.Selected != "Type B" {
		t.Fatalf("class = %q, body = %q", class.sel.Selected, body.sel.Selected)
	}
	if ed.demonsKilled.Text != "1" {
		t.Errorf("demons killed = %q", ed.demonsKilled.Text)
	}
	if ed.dirty {
		t.Error("loading marked the editor dirty")
	}
	if gift := choiceByName(t, ed, "PLAYER_STARTINGGIFT"); gift.sel.Selected != "Providential Ring" {
		t.Errorf("gift = %q", gift.sel.Selected)
	}
	if voice := choiceByName(t, ed, "PLAYER_VOICETYPE"); voice.sel.Selected != "Vocals D" {
		t.Errorf("voice = %q", voice.sel.Selected)
	}
	class.sel.SetSelected("Royalty")
	body.sel.SetSelected("Type A")
	choiceByName(t, ed, "0xA3C2D71A").sel.SetSelected("Type A")
	choiceByName(t, ed, "PLAYER_VOICETYPE").sel.SetSelected("Vocals F")
	typeInto(fieldByName(t, ed, "CLEARCOUNT"), "2")
	if !ed.dirty {
		t.Error("editor not marked dirty")
	}
	ed.saveInPlace()

	s := reload(t, path)
	for name, want := range map[string]string{
		"PLAYER_ARCHETYPE": "9",
		"PLAYER_ISMALE":    "false",
		"CLEARCOUNT":       "2",
	} {
		if got := s.FindName(name).String(); got != want {
			t.Errorf("%s = %s, want %s", name, got, want)
		}
	}
	// Copies of body type, animation style and voice follow the main field.
	for _, c := range []struct {
		path []uint32
		want string
	}{
		{[]uint32{keyNotMale}, "1"},
		{[]uint32{keyAppearance, keyAppearanceMale}, "0"},
		{[]uint32{keyAnimationB}, "false"},
		{[]uint32{keyNotAnimationB}, "1"},
		{[]uint32{keyAppearance, keyAppearanceAnimA}, "1"},
		{[]uint32{save.Hash("PLAYER_VOICETYPE")}, "2"},
		{[]uint32{keyVoiceCopy}, "2"},
		{[]uint32{keyAppearance, keyAppearanceVoice}, "2"},
	} {
		if got := s.FindPath(c.path...).String(); got != c.want {
			t.Errorf("%08X = %s, want %s", c.path, got, c.want)
		}
	}
}

func TestUnknownChoiceValue(t *testing.T) {
	ed, _ := openSample(t)
	ed.save.FindName("PLAYER_ARCHETYPE").SetInt(42)
	ed.loadForm()
	class := choiceByName(t, ed, "PLAYER_ARCHETYPE")
	if class.sel.Selected != "Unknown (42)" {
		t.Errorf("selected = %q", class.sel.Selected)
	}
	class.sel.SetSelected("Thief")
	if got := ed.save.FindName("PLAYER_ARCHETYPE").String(); got != "7" {
		t.Errorf("archetype = %s, want 7", got)
	}
}

func TestNoFileLoaded(t *testing.T) {
	test.NewTempApp(t)
	ed := newEditor(test.NewTempWindow(t, nil))
	for _, f := range ed.fields {
		if !f.entry.Disabled() {
			t.Errorf("%s is editable with no file open", f.name)
		}
	}
	for _, c := range ed.choices {
		if !c.sel.Disabled() {
			t.Errorf("%s is editable with no file open", c.name)
		}
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
