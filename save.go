package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

// Item is one save field: 'u' uint32, 'f' float32, 'b' byte, 's' length-prefixed string
type Item struct {
	T byte
	U uint32
	F float32
	B byte
	S []byte
}

type Fields struct {
	Version   uint32
	HdrStart  int
	HdrEndV2  int
	Levels    int
	Stats     int
	Weapons   []int
	Table     map[string]int
	HasPlayer bool
}

type rd struct {
	d  []byte
	i  int
	bo binary.ByteOrder
}

var errShort = errors.New("the file is incomplete or not in the expected format")

func (r *rd) need(n int) error {
	if r.i+n > len(r.d) {
		return errShort
	}
	return nil
}
func (r *rd) u32() (Item, error) {
	if err := r.need(4); err != nil {
		return Item{}, err
	}
	v := r.bo.Uint32(r.d[r.i:])
	r.i += 4
	return Item{T: 'u', U: v}, nil
}
func (r *rd) f32() (Item, error) {
	if err := r.need(4); err != nil {
		return Item{}, err
	}
	v := math.Float32frombits(r.bo.Uint32(r.d[r.i:]))
	r.i += 4
	return Item{T: 'f', F: v}, nil
}
func (r *rd) u8() (Item, error) {
	if err := r.need(1); err != nil {
		return Item{}, err
	}
	v := r.d[r.i]
	r.i++
	return Item{T: 'b', B: v}, nil
}
func (r *rd) str() (Item, error) {
	if err := r.need(4); err != nil {
		return Item{}, err
	}
	n := int(r.bo.Uint32(r.d[r.i:]))
	if n < 1 || n > 4096 || r.i+4+n > len(r.d) || r.d[r.i+4+n-1] != 0 {
		return Item{}, fmt.Errorf("malformed string at offset %d", r.i)
	}
	v := append([]byte(nil), r.d[r.i+4:r.i+4+n]...)
	r.i += 4 + n
	return Item{T: 's', S: v}, nil
}

func (it Item) Str() string { return strings.TrimRight(string(it.S), "\x00") }

func parseCopy(d []byte, bo binary.ByteOrder) (items []Item, F Fields, err error) {
	r := &rd{d: d, bo: bo}
	F.Table = map[string]int{}
	var x Item
	add := func(fn func() (Item, error), n int) {
		for k := 0; k < n && err == nil; k++ {
			x, err = fn()
			if err == nil {
				items = append(items, x)
			}
		}
	}
	add(r.u32, 1)
	if err != nil {
		return
	}
	F.Version = items[0].U
	if F.Version != 2 && F.Version != 8 {
		return nil, F, fmt.Errorf("unsupported save version %d", F.Version)
	}
	add(r.str, 1)
	add(r.u32, 1)
	add(r.u8, 1)
	add(r.u32, 3)
	F.HdrStart = len(items)
	add(r.u32, 1)
	add(r.f32, 3)
	for k := 0; k < 2; k++ {
		add(r.f32, 1)
		add(r.u32, 4)
	}
	F.HdrEndV2 = len(items)
	if F.Version >= 8 {
		add(r.u32, 22)
		add(r.u32, 1)
		if err != nil {
			return
		}
		tc := int(items[len(items)-1].U)
		if tc > 64 {
			return nil, F, errShort
		}
		for k := 0; k < tc && err == nil; k++ {
			add(r.str, 1)
			if err == nil {
				F.Table[items[len(items)-1].Str()] = len(items) - 1
			}
			add(r.u32, 11)
		}
	}
	F.Levels = len(items)
	add(r.u32, 1)
	if err != nil {
		return
	}
	nl := int(items[len(items)-1].U)
	if nl > 10000 {
		return nil, F, errShort
	}
	for k := 0; k < nl && err == nil; k++ {
		add(r.str, 1)
		add(r.u8, 2)
	}
	add(r.u32, 1)
	if err != nil {
		return
	}
	na := int(items[len(items)-1].U)
	if na > 1000 {
		return nil, F, errShort
	}
	for a := 0; a < na && err == nil; a++ {
		add(r.str, 2)
		if err != nil {
			return
		}
		cls := items[len(items)-1].Str()
		switch {
		case strings.HasSuffix(cls, "WS_PlayerController"):
			F.HasPlayer = true
			add(r.u32, 1)
			add(r.f32, 3)
			add(r.u32, 4)
			F.Stats = len(items)
			add(r.u32, 16)
			add(r.u32, 12)
			add(r.u8, 1)
			add(r.u32, 1)
			if err != nil {
				return
			}
			wc := int(items[len(items)-1].U)
			if wc > 64 {
				return nil, F, errShort
			}
			for k := 0; k < wc && err == nil; k++ {
				F.Weapons = append(F.Weapons, len(items))
				add(r.str, 2)
				add(r.u32, 9)
			}
			add(r.u32, 2)
			add(r.str, 1)
			add(r.u32, 1)
			add(r.u8, 3)
			add(r.f32, 2)
			add(r.u8, 3)
			add(r.u32, 1)
		case strings.HasSuffix(cls, "WS_PuzzleBox"):
			add(r.u32, 2)
			add(r.u8, 1)
		default:
			return nil, F, fmt.Errorf("unknown element in save: %s", cls)
		}
	}
	if err != nil {
		return
	}
	if r.i != len(d) {
		return nil, F, fmt.Errorf("structure mismatch (%d of %d bytes)", r.i, len(d))
	}
	if !F.HasPlayer {
		return nil, F, errors.New("the save contains no player data")
	}
	return
}

func serialize(items []Item, bo binary.ByteOrder) []byte {
	var b bytes.Buffer
	t := make([]byte, 4)
	for _, it := range items {
		switch it.T {
		case 'u':
			bo.PutUint32(t, it.U)
			b.Write(t)
		case 'f':
			bo.PutUint32(t, math.Float32bits(it.F))
			b.Write(t)
		case 'b':
			b.WriteByte(it.B)
		case 's':
			bo.PutUint32(t, uint32(len(it.S)))
			b.Write(t)
			b.Write(it.S)
		}
	}
	return b.Bytes()
}

// ---------- containers ----------

type pcFile struct {
	hdr  []byte // 20 bytes
	c    [2][]byte
	rest []byte
}

func isPC(b []byte) bool { return len(b) >= 24 && string(b[:4]) == "EESF" }

func splitCopies(d []byte, bo binary.ByteOrder) (c [2][]byte, rest []byte, err error) {
	p := 0
	for k := 0; k < 2; k++ {
		if p+4 > len(d) {
			return c, nil, errShort
		}
		n := int(bo.Uint32(d[p:]))
		if n <= 0 || p+4+n > len(d) {
			return c, nil, errShort
		}
		c[k] = d[p+4 : p+4+n]
		p += 4 + n
	}
	return c, d[p:], nil
}

func readPC(b []byte) (*pcFile, error) {
	if !isPC(b) {
		return nil, errors.New("not a PC save (missing EESF header)")
	}
	c, rest, err := splitCopies(b[20:], binary.LittleEndian)
	if err != nil {
		return nil, err
	}
	return &pcFile{hdr: append([]byte(nil), b[:20]...), c: c, rest: append([]byte(nil), rest...)}, nil
}

func buildPC(c1, c2 []byte, rest []byte, filetime uint64) []byte {
	le := binary.LittleEndian
	var data bytes.Buffer
	t := make([]byte, 4)
	for _, c := range [][]byte{c1, c2} {
		le.PutUint32(t, uint32(len(c)))
		data.Write(t)
		data.Write(c)
	}
	data.Write(rest)
	h := make([]byte, 20)
	copy(h, "EESF")
	le.PutUint32(h[8:], uint32(data.Len()))
	le.PutUint64(h[12:], filetime)
	return append(h, data.Bytes()...)
}

func nowFiletime() uint64 {
	return uint64(time.Now().UnixNano()/100) + 116444736000000000
}

// ---------- Xbox 360 -> PC conversion ----------

func v8Extra(items []Item, F Fields) []Item {
	ex := []Item{{T: 'u'}, {T: 'u'}, {T: 'u', U: 100}, {T: 'u'}, {T: 'u'}, {T: 'u'}, {T: 'u'}}
	for k := 0; k < 15; k++ {
		ex = append(ex, Item{T: 'u', U: items[F.Stats+k].U})
	}
	ex = append(ex, Item{T: 'u', U: uint32(len(F.Weapons))})
	for _, w := range F.Weapons {
		cls := items[w].Str()
		short := cls[strings.LastIndex(cls, ".")+1:]
		r := make([]uint32, 9)
		for j := 0; j < 9; j++ {
			r[j] = items[w+2+j].U
		}
		rec := []uint32{r[0], r[1], r[2], r[3], r[4], r[6], r[6], r[7], r[8], r[7], r[8]}
		ex = append(ex, Item{T: 's', S: append([]byte(short), 0)})
		for _, v := range rec {
			ex = append(ex, Item{T: 'u', U: v})
		}
	}
	return ex
}

func convert360(b []byte) ([]byte, error) {
	if isPC(b) {
		return nil, errors.New("this file is already a PC save")
	}
	c, _, err := splitCopies(b, binary.BigEndian)
	if err != nil {
		return nil, errors.New("this does not look like a Shadows of the Damned Xbox 360 save")
	}
	items, F, err := parseCopy(c[0], binary.BigEndian)
	if err != nil {
		return nil, fmt.Errorf("could not read the Xbox 360 save: %v", err)
	}
	if F.Version != 2 {
		return nil, fmt.Errorf("unexpected version %d in Xbox 360 save (expected 2)", F.Version)
	}
	out := append([]Item{}, items[:F.HdrEndV2]...)
	out = append(out, v8Extra(items, F)...)
	out = append(out, items[F.Levels:]...)
	out[0].U = 8
	body := serialize(out, binary.LittleEndian)
	if _, _, err := parseCopy(body, binary.LittleEndian); err != nil {
		return nil, fmt.Errorf("internal error while verifying the conversion: %v", err)
	}
	return buildPC(body, body, []byte{0, 0, 0, 0}, nowFiletime()), nil
}

// ---------- editing ----------

type Weapon struct {
	Cls  string    `json:"cls"`
	Name string    `json:"name"`
	Up   [3]uint32 `json:"up"`
}
type State struct {
	Map     string   `json:"map"`
	Time    float64  `json:"time"`
	Gems    uint32   `json:"gems"`
	Weapons []Weapon `json:"weapons"`
}

func isFirearm(cls string) bool {
	return strings.Contains(cls, "Gun") || strings.Contains(cls, "Assault")
}

func readState(b []byte) (State, error) {
	pf, err := readPC(b)
	if err != nil {
		return State{}, err
	}
	items, F, err := parseCopy(pf.c[0], binary.LittleEndian)
	if err != nil {
		return State{}, err
	}
	if F.Version != 8 {
		return State{}, fmt.Errorf("unsupported version %d (expected 8)", F.Version)
	}
	st := State{Map: items[1].Str(), Time: float64(items[F.HdrStart+4].F), Gems: items[F.Stats+1].U}
	for _, w := range F.Weapons {
		cls := items[w].Str()
		if !isFirearm(cls) {
			continue
		}
		st.Weapons = append(st.Weapons, Weapon{Cls: cls, Name: items[w+1].Str(),
			Up: [3]uint32{items[w+3].U, items[w+4].U, items[w+5].U}})
	}
	return st, nil
}

func applyState(b []byte, st State) ([]byte, error) {
	pf, err := readPC(b)
	if err != nil {
		return nil, err
	}
	var outc [2][]byte
	for k := 0; k < 2; k++ {
		items, F, err := parseCopy(pf.c[k], binary.LittleEndian)
		if err != nil {
			return nil, err
		}
		items[F.Stats+1].U = st.Gems
		items[F.HdrStart+6].U = st.Gems
		items[F.HdrEndV2+8].U = st.Gems
		for _, w := range st.Weapons {
			for _, wi := range F.Weapons {
				if items[wi].Str() != w.Cls {
					continue
				}
				short := w.Cls[strings.LastIndex(w.Cls, ".")+1:]
				t, ok := F.Table[short]
				for j := 0; j < 3; j++ {
					items[wi+3+j].U = w.Up[j]
					if ok {
						items[t+2+j].U = w.Up[j]
					}
				}
			}
		}
		outc[k] = serialize(items, binary.LittleEndian)
	}
	ft := binary.LittleEndian.Uint64(pf.hdr[12:])
	out := buildPC(outc[0], outc[1], pf.rest, ft)
	chk, err := readState(out)
	if err != nil || chk.Gems != st.Gems {
		return nil, errors.New("verification of the edited file failed")
	}
	return out, nil
}
