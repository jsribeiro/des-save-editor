package save

import (
	"bytes"
	"os"
	"testing"
)

// Sample saves live in samples/; tests skip if they are absent.
var samples = []string{"../../samples/USR-DATA", "../../samples/USR-DATA-2", "../../samples/USR-DATA-3"}

func loadSample(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("sample not available: %v", err)
	}
	return b
}

func TestHash(t *testing.T) {
	if got := Hash("PLAYER_SOULS"); got != 0x76FD62AF {
		t.Fatalf("Hash(PLAYER_SOULS) = %08x", got)
	}
}

func TestRoundTrip(t *testing.T) {
	for _, path := range samples {
		orig := loadSample(t, path)
		s, err := Parse(orig)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if out := s.Bytes(); !bytes.Equal(out, orig) {
			t.Fatalf("%s: re-serialized save differs (%d vs %d bytes)", path, len(out), len(orig))
		}
	}
}

func TestKnownValues(t *testing.T) {
	s, err := Parse(loadSample(t, samples[1]))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"PLAYER_NAME":            "John Souls",
		"PLAYER_SOULS":           "4450",
		"PLAYER_CUMULATIVESOULS": "9301",
		"PLAYER_CURRENTMAXHP":    "446",
		"PLAYER_BASESTRENGTH":    "16",
		"PLAYER_TOTALADDPARAM":   "90",
	}
	for name, v := range want {
		r := s.FindName(name)
		if r == nil {
			t.Errorf("%s not found", name)
		} else if r.String() != v {
			t.Errorf("%s = %q, want %q", name, r.String(), v)
		}
	}
}

func TestEditAndResize(t *testing.T) {
	orig := loadSample(t, samples[0])
	s, err := Parse(orig)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.FindName("PLAYER_SOULS").Set("123456"); err != nil {
		t.Fatal(err)
	}
	if err := s.FindName("PLAYER_NAME").Set("A Much Longer Name"); err != nil {
		t.Fatal(err)
	}
	if err := s.FindName("PLAYER_SOULS").Set("-1"); err == nil {
		t.Error("negative value accepted for u32")
	}
	out := s.Bytes()
	if len(out) != len(orig)+8 {
		t.Errorf("size %d, want %d", len(out), len(orig)+8)
	}
	s2, err := Parse(out) // also verifies length and checksum
	if err != nil {
		t.Fatal(err)
	}
	if v := s2.FindName("PLAYER_SOULS").String(); v != "123456" {
		t.Errorf("souls = %s", v)
	}
	if v := s2.FindName("PLAYER_NAME").String(); v != "A Much Longer Name" {
		t.Errorf("name = %s", v)
	}
	// The original buffer must be untouched.
	if _, err := Parse(orig); err != nil {
		t.Errorf("original buffer modified: %v", err)
	}
}

// New characters made to map character creation choices; see samples/README.md.
func TestCharacterCreationSamples(t *testing.T) {
	for _, c := range []struct{ file, archetype, isMale, gift, voice string }{
		{"A", "0", "false", "0", "3"}, // Soldier, Type A, no gift, 1st voice
		{"B", "9", "true", "9", "2"},  // Royalty, Type B, last gift, last voice
		{"C", "4", "false", "1", "4"}, // Magician, Type A, 1st gift, 2nd voice
		{"D", "7", "true", "2", "5"},  // Thief, Type B, 2nd gift, 3rd voice
		{"E", "8", "false", "3", "0"}, // Temple Knight, Type A (animation B), 3rd gift, 4th voice
	} {
		s, err := Parse(loadSample(t, "../../samples/RE/USR-DATA-"+c.file))
		if err != nil {
			t.Fatalf("%s: %v", c.file, err)
		}
		if out := s.Bytes(); !bytes.Equal(out, loadSample(t, "../../samples/RE/USR-DATA-"+c.file)) {
			t.Errorf("%s: re-serialized save differs", c.file)
		}
		for name, want := range map[string]string{
			"PLAYER_NAME":         c.file,
			"PLAYER_ARCHETYPE":    c.archetype,
			"PLAYER_ISMALE":       c.isMale,
			"PLAYER_STARTINGGIFT": c.gift,
			"PLAYER_VOICETYPE":    c.voice,
		} {
			if got := s.FindName(name).String(); got != want {
				t.Errorf("%s: %s = %s, want %s", c.file, name, got, want)
			}
		}
	}
}
