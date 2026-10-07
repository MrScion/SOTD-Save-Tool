# SOTD Save Tool

A small Windows tool for **Shadows of the Damned: Hella Remastered** (Steam):

- Converts **Xbox 360** saves (`.sav`, format version 2) to the remaster's PC format (version 8).
- Edits **white gems** and **weapon upgrades** in a PC save.
- Backs up your current save, with the date in its name, before writing to the game folder.

## Usage

1. Download `SOTD-Save-Tool.exe` from the *Releases* section.
2. Run it: a console window opens and the interface loads in your browser (only reachable from your own computer).
3. Load your PC save, or open a PC or Xbox 360 `.sav` file (Xbox 360 saves are converted automatically).
4. Edit the values and click **Save to game folder** with the game closed.

Save folder: `%USERPROFILE%\Saved Games\Shadows Of The Damned\`

## Status

- Conversion tested with a real Xbox 360 save: it loads correctly in the Steam version.
- Values confirmed in game: white gems and the three upgrade levels of each weapon.
- Not verified: the maximum level of each upgrade (the tool allows 0–5), and Xbox 360 saves containing level elements other than the ones analysed.
- The executable is not code-signed, so Windows SmartScreen may show a warning.

## Building

Requires Go 1.22 or later, no external dependencies:

```
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o SOTD-Save-Tool.exe .
```

## Disclaimer

Unofficial project, not affiliated with the game's developers or publishers. Always back up your saves before using it.
