package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

const maxRows = 6

var names = map[string]string{
	"WS_Weapon_Gun": "Boner", "WS_Weapon_Gun_Level2": "Hot Boner",
	"WS_Weapon_Assault": "Teether", "WS_Weapon_Assault_Level2": "Teethgrinder",
	"WS_Weapon_ShotGun": "Skullblaster", "WS_Weapon_ShotGun_Level2": "Skullfest",
}

type row struct {
	label *walk.Label
	edits [3]*walk.NumberEdit
}

type app struct {
	mw                              *walk.MainWindow
	fileLbl, levelLbl, timeLbl, msg *walk.Label
	convLbl                         *walk.Label
	gems                            *walk.NumberEdit
	rows                            [maxRows]row
	editBox                         *walk.Composite
	saveBtn, saveAsBtn, undoBtn     *walk.PushButton
	current                         []byte
	orig                            State
	loaded                          bool
}

func saveDir() string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, "Saved Games", "Shadows Of The Damned")
}
func savePath() string { return filepath.Join(saveDir(), "AUTOSAVE0.sav") }

func shortName(cls string) string { return cls[strings.LastIndex(cls, ".")+1:] }

func kind(cls string) string {
	switch {
	case strings.Contains(cls, "ShotGun"):
		return "shotgun"
	case strings.Contains(cls, "Assault"):
		return "rifle"
	}
	return "pistol"
}

func (a *app) errorBox(msg string) {
	walk.MsgBox(a.mw, "SOTD Save Tool", msg, walk.MsgBoxIconError|walk.MsgBoxOK)
}

func (a *app) load(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		a.errorBox("Could not read the file:\n" + err.Error())
		return
	}
	converted := false
	if !isPC(b) {
		pc, err := convert360(b)
		if err != nil {
			a.errorBox(err.Error())
			return
		}
		b, converted = pc, true
	}
	st, err := readState(b)
	if err != nil {
		a.errorBox(err.Error())
		return
	}
	a.current, a.orig, a.loaded = b, st, true
	a.fileLbl.SetText(path)
	a.convLbl.SetVisible(converted)
	a.show(st)
	a.setEnabled(true)
	if converted {
		a.msg.SetText("Xbox 360 save converted. Not saved yet.")
	} else {
		a.msg.SetText("Save loaded.")
	}
}

func (a *app) show(st State) {
	a.levelLbl.SetText(st.Map)
	s := int(st.Time + 0.5)
	a.timeLbl.SetText(fmt.Sprintf("%d h %d min", s/3600, s%3600/60))
	a.gems.SetValue(float64(st.Gems))
	for i := range a.rows {
		r := a.rows[i]
		vis := i < len(st.Weapons)
		r.label.SetVisible(vis)
		for j := 0; j < 3; j++ {
			r.edits[j].SetVisible(vis)
		}
		if !vis {
			continue
		}
		w := st.Weapons[i]
		n := names[shortName(w.Cls)]
		if n == "" {
			n = shortName(w.Cls)
		}
		r.label.SetText(n + " (" + kind(w.Cls) + ")")
		for j := 0; j < 3; j++ {
			r.edits[j].SetValue(float64(w.Up[j]))
		}
	}
}

func (a *app) collect() State {
	st := a.orig
	st.Weapons = append([]Weapon(nil), a.orig.Weapons...)
	st.Gems = uint32(a.gems.Value())
	for i := range st.Weapons {
		if i >= maxRows {
			break
		}
		for j := 0; j < 3; j++ {
			st.Weapons[i].Up[j] = uint32(a.rows[i].edits[j].Value())
		}
	}
	return st
}

func (a *app) build() ([]byte, bool) {
	out, err := applyState(a.current, a.collect())
	if err != nil {
		a.errorBox(err.Error())
		return nil, false
	}
	return out, true
}

func (a *app) saveToGame() {
	if !a.loaded {
		return
	}
	if walk.MsgBox(a.mw, "Save to game folder",
		"The save will be written to:\n"+savePath()+"\n\nYour current AUTOSAVE0.sav will be backed up first.\n\nIs the game closed?",
		walk.MsgBoxIconQuestion|walk.MsgBoxYesNo) != walk.DlgCmdYes {
		return
	}
	out, ok := a.build()
	if !ok {
		return
	}
	if err := os.MkdirAll(saveDir(), 0755); err != nil {
		a.errorBox("Cannot create the save folder:\n" + err.Error())
		return
	}
	backup := ""
	if old, err := os.ReadFile(savePath()); err == nil {
		backup = savePath() + ".backup-" + time.Now().Format("20060102-150405")
		if err := os.WriteFile(backup, old, 0644); err != nil {
			a.errorBox("Backup failed, nothing was saved:\n" + err.Error())
			return
		}
	}
	if err := os.WriteFile(savePath(), out, 0644); err != nil {
		a.errorBox("Could not write the save:\n" + err.Error())
		return
	}
	a.current, a.orig = out, a.collect()
	a.convLbl.SetVisible(false)
	m := "Saved to the game folder."
	if backup != "" {
		m += " Backup: " + filepath.Base(backup)
	}
	a.msg.SetText(m)
}

func (a *app) saveAs() {
	if !a.loaded {
		return
	}
	out, ok := a.build()
	if !ok {
		return
	}
	dlg := &walk.FileDialog{Title: "Save as", Filter: "Save files (*.sav)|*.sav|All files (*.*)|*.*", FilePath: "AUTOSAVE0.sav"}
	if ok, _ := dlg.ShowSave(a.mw); !ok {
		return
	}
	p := dlg.FilePath
	if filepath.Ext(p) == "" {
		p += ".sav"
	}
	if err := os.WriteFile(p, out, 0644); err != nil {
		a.errorBox("Could not write the file:\n" + err.Error())
		return
	}
	a.msg.SetText("Saved to " + p)
}

func (a *app) openFile() {
	dlg := &walk.FileDialog{Title: "Open save (PC or Xbox 360)", Filter: "Save files (*.sav)|*.sav|All files (*.*)|*.*"}
	if _, err := os.Stat(saveDir()); err == nil {
		dlg.InitialDirPath = saveDir()
	}
	if ok, _ := dlg.ShowOpen(a.mw); ok {
		a.load(dlg.FilePath)
	}
}

func (a *app) setEnabled(on bool) {
	a.editBox.SetEnabled(on)
	a.saveBtn.SetEnabled(on)
	a.saveAsBtn.SetEnabled(on)
	a.undoBtn.SetEnabled(on)
}

func upgradeEdit(target **walk.NumberEdit) Widget {
	return NumberEdit{AssignTo: target, MinValue: 0, MaxValue: 5, Decimals: 0, SpinButtonsVisible: true, MaxSize: Size{Width: 70}}
}

func main() {
	a := &app{}
	grid := []Widget{
		Label{Text: "Weapon", Font: Font{Bold: true}},
		Label{Text: "Upgrade 1", Font: Font{Bold: true}},
		Label{Text: "Upgrade 2", Font: Font{Bold: true}},
		Label{Text: "Upgrade 3", Font: Font{Bold: true}},
	}
	for i := 0; i < maxRows; i++ {
		grid = append(grid, Label{AssignTo: &a.rows[i].label, Visible: false})
		for j := 0; j < 3; j++ {
			e := upgradeEdit(&a.rows[i].edits[j])
			ne := e.(NumberEdit)
			ne.Visible = false
			grid = append(grid, ne)
		}
	}
	_, gameExists := os.Stat(savePath())

	err := MainWindow{
		AssignTo: &a.mw,
		Title:    "SOTD Save Tool",
		MinSize:  Size{Width: 520, Height: 520},
		Size:     Size{Width: 560, Height: 580},
		Layout:   VBox{Margins: Margins{Left: 14, Top: 12, Right: 14, Bottom: 12}, Spacing: 10},
		Children: []Widget{
			Label{Text: "Shadows of the Damned: Hella Remastered", Font: Font{PointSize: 12, Bold: true}},
			Label{Text: "Convert Xbox 360 saves to PC and edit white gems and weapon upgrades."},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				PushButton{Text: "Load my PC save", Enabled: gameExists == nil, OnClicked: func() { a.load(savePath()) }},
				PushButton{Text: "Open .sav file (PC or Xbox 360)...", OnClicked: a.openFile},
				HSpacer{},
			}},
			Label{Text: "Save folder: " + saveDir(), TextColor: walk.RGB(110, 110, 110)},
			Label{AssignTo: &a.convLbl, Visible: false, Font: Font{Bold: true}, TextColor: walk.RGB(190, 20, 110),
				Text: "Xbox 360 save converted to the PC format. Check the values and save it."},
			GroupBox{Title: "Save", Layout: Grid{Columns: 2}, Children: []Widget{
				Label{Text: "File:"}, Label{AssignTo: &a.fileLbl, Text: "-"},
				Label{Text: "Level:"}, Label{AssignTo: &a.levelLbl, Text: "-"},
				Label{Text: "Play time:"}, Label{AssignTo: &a.timeLbl, Text: "-"},
			}},
			Composite{AssignTo: &a.editBox, Enabled: false, Layout: VBox{MarginsZero: true}, Children: []Widget{
				GroupBox{Title: "White gems", Layout: HBox{}, Children: []Widget{
					NumberEdit{AssignTo: &a.gems, MinValue: 0, MaxValue: 99999, Decimals: 0, SpinButtonsVisible: true, MaxSize: Size{Width: 110}},
					HSpacer{},
				}},
				GroupBox{Title: "Weapon upgrades", Layout: Grid{Columns: 4}, Children: grid},
				Label{Text: "Columns follow the order the game shows (e.g. 3/2/0). The maximum level is unconfirmed; 0 to 5 allowed.",
					TextColor: walk.RGB(110, 110, 110)},
			}},
			VSpacer{},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				PushButton{AssignTo: &a.saveBtn, Text: "Save to game folder", Enabled: false, OnClicked: a.saveToGame},
				PushButton{AssignTo: &a.saveAsBtn, Text: "Save as...", Enabled: false, OnClicked: a.saveAs},
				PushButton{AssignTo: &a.undoBtn, Text: "Undo changes", Enabled: false, OnClicked: func() { a.show(a.orig); a.msg.SetText("Changes undone.") }},
				HSpacer{},
			}},
			Label{AssignTo: &a.msg, Text: "Open a save to start."},
		},
	}.Create()
	if err != nil {
		walk.MsgBox(nil, "SOTD Save Tool", "Could not start: "+err.Error(), walk.MsgBoxIconError)
		return
	}
	a.mw.Run()
}
