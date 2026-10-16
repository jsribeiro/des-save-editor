package save

import "fmt"

// Field names whose FNV-1a hash matches a key in known saves.
var knownNames = []string{
	"PLAYTIME",
	"PLAYER_NAME",
	"PLAYER_SOULS",
	"PLAYER_CUMULATIVESOULS",
	"PLAYER_TOTALADDPARAM",
	"CHARACTER_NEW_ARCHSTONE",
	"CLEARCOUNT",
	"DEMONSKILLED",

	"PLAYER_ARCHETYPE", "PLAYER_ISMALE", "PLAYER_VOICETYPE", "PLAYER_STARTINGGIFT",

	"PLAYER_BASEMAXHP", "PLAYER_CURRENTMAXHP", "PLAYER_CURRENTHP",
	"PLAYER_BASEMAXMP", "PLAYER_CURRENTMAXMP", "PLAYER_CURRENTMP",
	"PLAYER_BASEMAXSP", "PLAYER_CURRENTMAXSP", "PLAYER_CURRENTSP",
}

// Attributes in the order the game's status screen lists them.
var Attributes = []string{
	"VITALITY", "INTELLIGENCE", "ENDURANCE", "STRENGTH",
	"DEXTERITY", "MAGIC", "FAITH", "LUCK",
}

var nameByKey = map[uint32]string{}

func init() {
	for _, a := range Attributes {
		knownNames = append(knownNames, "PLAYER_BASE"+a, "PLAYER_CURRENT"+a)
	}
	for _, n := range knownNames {
		nameByKey[Hash(n)] = n
	}
}

// Hash returns the record key for a field name.
func Hash(name string) uint32 { return Checksum([]byte(name)) }

// NameOf returns the known field name for a key.
func NameOf(key uint32) (string, bool) {
	n, ok := nameByKey[key]
	return n, ok
}

// Label returns the field name for a key, or the key in hex if unknown.
func Label(key uint32) string {
	if n, ok := nameByKey[key]; ok {
		return n
	}
	return fmt.Sprintf("0x%08X", key)
}
