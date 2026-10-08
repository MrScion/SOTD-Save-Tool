# SOTD Save Tool

A native Windows tool for **Shadows of the Damned: Hella Remastered** (Steam):

- Converts **Xbox 360** saves (`.sav`, format version 2) to the remaster's PC format (version 8).
- Edits **white gems** and **weapon upgrades** in a PC save.
- Backs up your current save, with the date in its name, before writing to the game folder.

## Usage

1. Download `SOTD-Save-Tool.exe` from the *Releases* section.
2. Run it. No installation needed.
3. Click **Load my PC save**, or **Open .sav file** to pick a PC or Xbox 360 save (Xbox 360 saves are converted automatically).
4. Edit the values and click **Save to game folder** with the game closed, or **Save as...** to write the file elsewhere.

Save folder: `%USERPROFILE%\Saved Games\Shadows Of The Damned\`

## Status

- Conversion tested with a real Xbox 360 save: it loads correctly in the Steam version.
- Values confirmed in game: white gems and the three upgrade levels of each weapon.
- Not verified: the maximum level of each upgrade (the tool allows 0–5), and Xbox 360 saves containing level elements other than the ones analysed.
- The executable is not code-signed, so Windows SmartScreen may show a warning.

## Building

Requires Go 1.22 or later. The interface uses [lxn/walk](https://github.com/lxn/walk) (native Win32 controls, no CGO).

```
go install github.com/akavel/rsrc@latest
rsrc -manifest app.manifest -ico app.ico -arch amd64 -o rsrc_windows_amd64.syso
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -H windowsgui" -o SOTD-Save-Tool.exe .
```

## Disclaimer

Unofficial project, not affiliated with the game's developers or publishers. Always back up your saves before using it.
