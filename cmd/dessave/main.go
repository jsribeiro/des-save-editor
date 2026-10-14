// Command dessave dumps and edits Demon's Souls (2020) USR-DATA saves from the command line.
//
//	dessave dump FILE
//	dessave get  FILE FIELD
//	dessave set  FILE FIELD VALUE
//	dessave hash NAME...
//
// FIELD is a known name (PLAYER_SOULS) or a hex key (0x76FD62AF). set edits
// top-level fields in place and writes a timestamped backup first.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jsribeiro/des-save-editor/internal/save"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "dessave:", err)
		os.Exit(1)
	}
}

func usage() error {
	return fmt.Errorf("usage:\n  dessave dump FILE\n  dessave get FILE FIELD\n  dessave set FILE FIELD VALUE\n  dessave hash NAME...")
}

func run(args []string) error {
	if len(args) < 2 {
		return usage()
	}
	if args[0] == "hash" {
		for _, n := range args[1:] {
			fmt.Printf("0x%08X  %s\n", save.Hash(n), n)
		}
		return nil
	}
	path := args[1]
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	s, err := save.Parse(b)
	if err != nil {
		return err
	}
	switch {
	case args[0] == "dump" && len(args) == 2:
		s.Walk(func(r *save.Record, depth int) {
			fmt.Printf("%06X  %s%-28s %-6s %s\n", r.Offset, strings.Repeat("  ", depth), save.Label(r.Key), r.TypeName(), r.String())
		})
		return nil
	case args[0] == "get" && len(args) == 3:
		r, err := find(s, args[2])
		if err != nil {
			return err
		}
		fmt.Println(r.String())
		return nil
	case args[0] == "set" && len(args) == 4:
		r, err := find(s, args[2])
		if err != nil {
			return err
		}
		old := r.String()
		if err := r.Set(args[3]); err != nil {
			return err
		}
		backup := fmt.Sprintf("%s.%s.bak", path, time.Now().Format("20060102-150405"))
		if err := os.WriteFile(backup, b, 0o644); err != nil {
			return fmt.Errorf("writing backup: %w", err)
		}
		if err := os.WriteFile(path, s.Bytes(), 0o644); err != nil {
			return err
		}
		fmt.Printf("%s: %s -> %s (backup: %s)\n", save.Label(r.Key), old, r.String(), backup)
		return nil
	}
	return usage()
}

func find(s *save.Save, field string) (*save.Record, error) {
	key := save.Hash(field)
	if h, ok := strings.CutPrefix(strings.ToLower(field), "0x"); ok {
		v, err := strconv.ParseUint(h, 16, 32)
		if err != nil {
			return nil, fmt.Errorf("bad key %q", field)
		}
		key = uint32(v)
	}
	r := s.Find(key)
	if r == nil {
		return nil, fmt.Errorf("no top-level field %s", field)
	}
	return r, nil
}
