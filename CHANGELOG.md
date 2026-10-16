# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- **Character creation fields**, each as a drop-down: class, body type, animation style, voice and
  starting gift (with the gift's quantity, weight and in-game description).
- **Progress**: New Game cycle (editable) and demons killed.
- The game keeps copies of body type, animation style and voice in other fields, some inverted and
  some inside the character-creator object. Editing one of these choices updates every copy.
- New known field names: `PLAYER_ARCHETYPE`, `PLAYER_ISMALE`, `PLAYER_VOICETYPE`,
  `PLAYER_STARTINGGIFT`, `CLEARCOUNT` and `DEMONSKILLED`. `dessave` and the All fields tab show them.
- Five character creation saves in `samples/characters/` that map these fields, documented in
  `samples/README.md`, with each class's starting attributes, HP, MP and stamina.

### Changed

- The window is split into **Character**, **Stats** and **All fields** tabs. Stats holds the
  attributes and HP / MP / stamina.
- Readability: help text and read-only values have more contrast, valid fields no longer show a tick,
  the Stats grids no longer stretch short numbers across the window, and card titles are smaller.
- The window opens at 880 × 940, large enough to show each tab without scrolling.

### Fixed

- `make windows` failed on Windows because the path to the `fyne` tool lost its backslashes.

## [0.1.0] - 2026-10-14

First release.

### Added

- GUI editor for the decrypted `USR-DATA` save of Demon's Souls (2020):
  - **Character**: name, souls, total souls gained and play time.
  - **Attributes**: the eight attributes, base and current, limited to 1–99. By default, changing a
    base value also updates the current value (keeping equipment bonuses), the attribute total and
    the soul level.
  - **Souls**: raising souls raises total souls gained by the same amount, as in game.
  - **HP / MP / Stamina**: base max, current max and current values.
  - **All fields**: every record in the file, nested ones included, with a filter and editing of
    numbers, true/false values and text.
- Saving writes a timestamped backup first and recalculates the file's lengths and checksum.
- `dessave` command-line tool: `dump`, `get`, `set` and `hash`.
- Save format notes in `docs/FORMAT.md` and three sample saves.
- Builds for macOS (universal), Windows and Linux, published by GitHub Actions on tags.

[Unreleased]: https://github.com/jsribeiro/des-save-editor/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/jsribeiro/des-save-editor/releases/tag/v0.1.0
