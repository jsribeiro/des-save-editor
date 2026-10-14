# USR-DATA save format

What's known so far about the Demon's Souls (2020) `USR-DATA` save file. It was worked out by
comparing saves taken a few seconds apart; see [samples/README.md](../samples/README.md) for the
saves and the differences between them.

## Header

All values are little-endian.

| Offset | Size | Meaning |
|---|---|---|
| 0x00 | u32 | Version (7) |
| 0x04 | u32 | Checksum: FNV-1a 32 of every byte from 0x10 to the end |
| 0x08 | u32 | Length of the data from 0x10 to the end |
| 0x0C | u32 | Unknown (1) |
| 0x10 | … | Records |

## Records

Each record is `[u32 key][u8 type][value]`. The key is the FNV-1a 32 hash of an
UPPER_SNAKE_CASE field name, e.g. `PLAYER_SOULS` → `0x76FD62AF`.

| Type | Value |
|---|---|
| 0x02–0x05 | Signed int, 1/2/4/8 bytes (signedness is a guess) |
| 0x06–0x09 | Unsigned int, 1/2/4/8 bytes (signedness is a guess) |
| 0x0A / 0x0B | f32 / f64 |
| 0x0C | bool (1 byte) |
| 0x0D | String: LEB128 length + bytes |
| 0x0E | Blob: LEB128 length + bytes, often a nested record list |
| 0x0F | Four f32 (colours) |

The FNV-1a 32 hash starts at `0x811C9DC5`; for each byte it XORs the byte in, then multiplies by
`0x01000193` (keeping 32 bits). `dessave hash NAME` prints the key for a name.

## Known fields

Known field names are listed in [internal/save/names.go](../internal/save/names.go). To add one,
add the name to `knownNames`; the editor and `dessave` show it wherever its hash appears.

| Field | Meaning |
|---|---|
| `PLAYER_NAME` | Character name |
| `PLAYER_SOULS` | Souls held |
| `PLAYER_CUMULATIVESOULS` | Every soul ever gained; spending souls doesn't lower it |
| `PLAYTIME` | Play time in milliseconds (f32) |
| `PLAYER_BASE<ATTR>` / `PLAYER_CURRENT<ATTR>` | The eight attributes: VITALITY, INTELLIGENCE, ENDURANCE, STRENGTH, DEXTERITY, MAGIC, FAITH, LUCK. Current includes equipment bonuses. |
| `PLAYER_TOTALADDPARAM` | Sum of the eight attributes |
| `PLAYER_BASEMAXHP`, `PLAYER_CURRENTMAXHP`, `PLAYER_CURRENTHP` | HP |
| `PLAYER_BASEMAXMP`, `PLAYER_CURRENTMAXMP`, `PLAYER_CURRENTMP` | MP |
| `PLAYER_BASEMAXSP`, `PLAYER_CURRENTMAXSP`, `PLAYER_CURRENTSP` | Stamina |
| `CHARACTER_NEW_ARCHSTONE` | Unknown flag |

## Notes

- Soul level isn't stored. It is `PLAYER_TOTALADDPARAM − 80` (confirmed in game at soul levels 10
  and 79). Attributes go up to 99, so the maximum is 8 × 99 − 80 = 712.
- `PLAYER_BASEMAXHP` is the full max HP. `PLAYER_CURRENTMAXHP` is 70% of it in Soul Form.
- `0x6E704021`, `0x21164EAD` and `0x36553DB7` are probably the poison, bleed and plague resistance totals.
  They matched the status screen, and poison and bleed rose with Endurance.
- Inventory is a flat run of numbered slots, each with its own keys (index, category, item ID,
  quantity, …). Consumables use category `0x40000000`.
- The large blob `0x6D87F588` (≈450 KB) has its own, unknown layout. It holds the current level's
  state: in the Nexus it shrinks to a few KB, and the level string (`0x002DCAB1`), position and
  rotation blobs and a 74-entry `i16` list (count in `0xCC62FF39`) are removed or emptied.
- Keys in an indexed list (names differing only in their last character) can be grouped without
  knowing their names. Multiply a key by the inverse of the FNV prime (`0x01000193`) mod 2³². The
  result is the hash of the name minus its last character, XORed with that character, so keys that
  share a prefix agree in all but the low bits.
