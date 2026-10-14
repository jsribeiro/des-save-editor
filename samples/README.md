# Sample saves

Three `USR-DATA` saves from the same Demon's Souls (2020) character, taken a few seconds apart.
The tests use them (see [internal/save/save_test.go](../internal/save/save_test.go) and
[main_test.go](../main_test.go)). The format itself is described in [docs/FORMAT.md](../docs/FORMAT.md).

| File | Size | Checksum (0x04) | Data length (0x08) | SHA-256 |
|---|---|---|---|---|
| `USR-DATA` | 469,917 | `0xFF00ED87` | 469,901 | `66f01645…d871e1` |
| `USR-DATA-2` | 474,750 | `0xA88247F3` | 474,734 | `733effa6…7e54` |
| `USR-DATA-3` | 474,750 | `0xE27AC08F` | 474,734 | `869382ea…dcf1` |

All three have version 7 at 0x00 and 1 at 0x0C.

## The character

The in-game status screen (taken at the time of `USR-DATA-2`) matches these fields:

| Field | Value | In game |
|---|---|---|
| `PLAYER_NAME` | `John Souls` | Name |
| `PLAYER_SOULS` | 4450 | Souls |
| `PLAYER_TOTALADDPARAM` | 90 | Soul level 10 (90 − 80) |
| `PLAYER_BASEVITALITY` / `PLAYER_CURRENTVITALITY` | 13 / 13 | Vitality 13 |
| `PLAYER_BASEINTELLIGENCE` / `…CURRENT…` | 11 / 11 | Intelligence 11 |
| `PLAYER_BASEENDURANCE` / `…CURRENT…` | 12 / 12 | Endurance 12 |
| `PLAYER_BASESTRENGTH` / `…CURRENT…` | 16 / 16 | Strength 16 |
| `PLAYER_BASEDEXTERITY` / `…CURRENT…` | 10 / 10 | Dexterity 10 |
| `PLAYER_BASEMAGIC` / `…CURRENT…` | 10 / 10 | Magic 10 |
| `PLAYER_BASEFAITH` / `…CURRENT…` | 11 / 11 | Faith 11 |
| `PLAYER_BASELUCK` / `…CURRENT…` | 7 / 7 | Luck 7 |
| `PLAYER_BASEMAXHP` | 638 | — (full max HP) |
| `PLAYER_CURRENTMAXHP` / `PLAYER_CURRENTHP` | 446 / 446 | HP 446 / 446 (446 = 70% of 638, Soul Form) |
| `PLAYER_BASEMAXMP` / `PLAYER_CURRENTMAXMP` / `PLAYER_CURRENTMP` | 73 / 73 / 73 | MP 73 / 73 |
| `PLAYER_BASEMAXSP` / `PLAYER_CURRENTMAXSP` / `PLAYER_CURRENTSP` | 95 / 95 / 95 | Stamina 95 |

Other things visible in game but **not** found in the save: class (Knight), body type (Type B),
Souls Required (829), equip/item burden, item discovery, attack power, defence and resistances.
Most of these are probably calculated by the game; class and body type are probably stored under
keys we haven't named yet.

Other notable values, the same in all three files:

- A level or area string `spiderrails` (key `0x002DCAB1`).
- A 16-byte blob `0xFB0FAEB2` holding four floats (172.36, 0.199, 184.25, 1.0), probably the player position.
- A 16-byte blob `0x739273B8` holding (0, 0.477, 0, 0), probably the player rotation.
- `CHARACTER_NEW_ARCHSTONE` = 1.
- A 93-record object `0x639244AA` with colours and face-slider floats (the character creator).

## Inventory

The inventory is a flat run of numbered slots, about 8 records each. Every record has its own key
(keys aren't reused between slots). The two slots that changed:

| | Slot 16 | Slot 33 |
|---|---|---|
| Slot index | `0x5898C000` = 16 | `0x903D2855` = 33 |
| Category | `0x4EA8D3EC` = `0x40000000` | `0x7E13D697` = `0x40000000` |
| Item ID | `0x33F3730D` = 1025 | `0x7A689BAC` = 1026 |
| Quantity | `0x354B14D9` | `0x8E41F1AE` |
| Slot index again | `0x06ED3784` = 16 | `0x25323AE1` = 33 |
| Unknown | `0x1E751D67` = `0x00650010` | `0x7A70D95A` = `0x00660021` |

`0x40000000` seems to be the consumable category. Other slots use `0x10000000`, probably weapons or armour.
The unknown field's low 16 bits are the slot index; the high 16 bits (101, 102) are unknown.

## Differences

### `USR-DATA` → `USR-DATA-2`: used one soul item worth 200 (item 1025, "Unknown Warrior's Soul"?)

| Field | Before | After |
|---|---|---|
| `PLAYER_SOULS` | 4250 | 4450 (+200) |
| `PLAYER_CUMULATIVESOULS` | 9101 | 9301 (+200) |
| Slot 16 quantity (`0x354B14D9`) | 11 | 10 |
| `PLAYTIME` (ms) | 8,578,000 | 8,659,000 (+81 s) |
| Position Y (`0xFB0FAEB2`) | 0.19921902 | 0.19921890 (rounding-level change, not real movement) |
| Blob `0x522AD995` (2,256 bytes) | `3f 04 00 08 0a ac 02` at byte 25, `f7` at byte 35 | zeros, `ff` at byte 35 |
| Big blob `0x6D87F588` | 452,259 bytes, count 443 | 457,092 bytes, count 470 (+27) |

The big blob starts with two u32s (3, then a count). Apart from those first bytes and the last
12,468 bytes, most of its content differs, so it is rewritten rather than appended to. It's the whole
reason the file grew by 4,833 bytes. The change in `0x522AD995` may be "last used item" state.

### `USR-DATA-2` → `USR-DATA-3`: used one soul item worth 400 (item 1026)

| Field | Before | After |
|---|---|---|
| `PLAYER_SOULS` | 4450 | 4850 (+400) |
| `PLAYER_CUMULATIVESOULS` | 9301 | 9701 (+400) |
| Slot 33 quantity (`0x8E41F1AE`) | 6 | 5 |
| `PLAYTIME` (ms) | 8,659,000 | 8,744,000 (+85 s) |

Nothing else changed, not even the big blob, so these are the only fields that change when you use a soul item.
