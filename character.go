package main

import (
	"fmt"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/jsribeiro/des-save-editor/internal/save"
)

// Classes in character creation order; PLAYER_ARCHETYPE is the index (confirmed for 0, 1, 4, 7 and 9).
var classes = []string{
	"Soldier", "Knight", "Hunter", "Priest", "Magician",
	"Wanderer", "Barbarian", "Thief", "Temple Knight", "Royalty",
}

// Starting gifts in menu order; PLAYER_STARTINGGIFT is the index.
var gifts = []struct{ name, desc string }{
	{"None", ""},
	{"Augite of Guidance", "×10, weight 1.0. A stone that radiates a guiding light."},
	{"Grass Jelly", "×2, weight 2.0. Recovers some of user's HP over time."},
	{"Elixir", "×5, weight 1.5. Temporarily increases the rate of stamina recovery."},
	{"Bright Water", "×3, weight 3.0. Recovers some of the user's MP over time."},
	{"Providential Ring", "×1, weight 0.2. Raises item discovery."},
	{"Kunai", "×10, weight 2.0. Poison throwing weapon."},
	{"Fire Bomb", "×5, weight 1.5. Oil cask. Produces a large flame when thrown."},
	{"Soul Remains", "×10, weight 1.0. Serves as bait for the soul-starved."},
	{"Fire Arrow", "×10, weight 2.0. Inflict fire damage when equipped with a bow."},
}

// Voices in menu order (Vocals A–F) and their PLAYER_VOICETYPE values.
var (
	voices      = []string{"Vocals A", "Vocals B", "Vocals C", "Vocals D", "Vocals E", "Vocals F"}
	voiceValues = []string{"3", "4", "5", "0", "1", "2"}
)

// Unnamed keys for character creation choices and the copies the game keeps of them.
// The copies matched in every sample save, so the editor keeps them in step.
const (
	keyAppearance      = 0x639244AA // character creator object
	keyAnimationB      = 0xA3C2D71A // animation style: true for B
	keyNotMale         = 0xF3E6ADE6 // 1 for body Type A
	keyAppearanceMale  = 0x6B00F985 // in the creator object, 1 for body Type B
	keyNotAnimationB   = 0x62A15576 // 1 for animation style A
	keyAppearanceAnimA = 0xF5925F05 // in the creator object, 1 for animation style A
	keyVoiceCopy       = 0x33E2F887
	keyAppearanceVoice = 0xCE09E754 // in the creator object
)

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
	classValues := make([]string, len(classes))
	for i := range classes {
		classValues[i] = strconv.Itoa(i)
	}
	class := ed.newChoice(save.Hash("PLAYER_ARCHETYPE"), classes, classValues)
	body := ed.newChoice(save.Hash("PLAYER_ISMALE"), []string{"Type A", "Type B"}, []string{"false", "true"}).
		mirror([]string{"1", "0"}, keyNotMale).
		mirror([]string{"0", "1"}, keyAppearance, keyAppearanceMale)
	anim := ed.newChoice(keyAnimationB, []string{"Type A", "Type B"}, []string{"false", "true"}).
		mirror([]string{"1", "0"}, keyNotAnimationB).
		mirror([]string{"1", "0"}, keyAppearance, keyAppearanceAnimA)
	voice := ed.newChoice(save.Hash("PLAYER_VOICETYPE"), voices, voiceValues).
		mirror(voiceValues, keyVoiceCopy).
		mirror(voiceValues, keyAppearance, keyAppearanceVoice)
	giftNames, giftValues := make([]string, len(gifts)), make([]string, len(gifts))
	for i, g := range gifts {
		giftNames[i], giftValues[i] = g.name, strconv.Itoa(i)
	}
	gift := ed.newChoice(save.Hash("PLAYER_STARTINGGIFT"), giftNames, giftValues)
	giftDesc := hint("")
	gift.onChange = func() {
		desc := ""
		if i := gift.index(); i >= 0 {
			desc = gifts[i].desc
		}
		setHint(giftDesc, desc)
	}
	ed.playtime = widget.NewLabel("")
	character := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("Name", name.entry),
			widget.NewFormItem("Class", class.sel),
			widget.NewFormItem("Body type", body.sel),
			widget.NewFormItem("Animation style", anim.sel),
			widget.NewFormItem("Voice", voice.sel),
			widget.NewFormItem("Starting gift", container.NewVBox(gift.sel, giftDesc)),
			widget.NewFormItem("Play time", ed.playtime),
		),
		hint("Class and starting gift only change what the game records; attributes, equipment and items stay as they are."),
	)

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
	souls := widget.NewForm(
		widget.NewFormItem("Souls", ed.souls.entry),
		widget.NewFormItem("Total souls gained", ed.cumulative.entry),
		widget.NewFormItem("", ed.soulsSync),
	)

	ed.demonsKilled = widget.NewLabel("")
	progress := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("New Game cycle", ed.newField("CLEARCOUNT").entry),
			widget.NewFormItem("Demons killed", ed.demonsKilled),
		),
		hint("New Game cycle is 0 on the first playthrough, 1 on NG+, and so on. Editing it hasn't been tested in game yet."),
	)

	return container.NewVBox(
		widget.NewCard("Character", "", character),
		widget.NewCard("Souls", "", souls),
		widget.NewCard("Progress", "", progress),
	)
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
