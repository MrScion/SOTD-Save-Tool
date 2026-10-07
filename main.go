package main

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

//go:embed index.html
var assets embed.FS

var (
	mu      sync.Mutex
	current []byte // loaded PC save (already converted if it came from Xbox 360)
	token   string
)

func saveDir() string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, "Saved Games", "Shadows Of The Damned")
}
func savePath() string { return filepath.Join(saveDir(), "AUTOSAVE0.sav") }

func jsonOut(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, msg string) { jsonOut(w, 400, map[string]string{"error": msg}) }

func guard(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Token") != token {
			http.Error(w, "forbidden", 403)
			return
		}
		h(w, r)
	}
}

// loadBytes converts Xbox 360 saves to PC before loading
func loadBytes(b []byte) (map[string]interface{}, error) {
	converted := false
	if !isPC(b) {
		pc, err := convert360(b)
		if err != nil {
			return nil, err
		}
		b, converted = pc, true
	}
	st, err := readState(b)
	if err != nil {
		return nil, err
	}
	mu.Lock()
	current = b
	mu.Unlock()
	return map[string]interface{}{"state": st, "converted": converted}, nil
}

func backup() (string, error) {
	src := savePath()
	b, err := os.ReadFile(src)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	dst := filepath.Join(saveDir(), "AUTOSAVE0.sav.backup-"+time.Now().Format("20060102-150405"))
	return dst, os.WriteFile(dst, b, 0644)
}

func main() {
	rb := make([]byte, 16)
	rand.Read(rb)
	token = hex.EncodeToString(rb)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		page, _ := assets.ReadFile("index.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(strings.Replace(string(page), "__TOKEN__", token, 1)))
	})
	mux.HandleFunc("/api/info", guard(func(w http.ResponseWriter, r *http.Request) {
		_, err := os.Stat(savePath())
		jsonOut(w, 200, map[string]interface{}{"dir": saveDir(), "exists": err == nil})
	}))
	mux.HandleFunc("/api/open", guard(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil || len(b) == 0 {
			fail(w, "No file received.")
			return
		}
		res, err := loadBytes(b)
		if err != nil {
			fail(w, err.Error())
			return
		}
		jsonOut(w, 200, res)
	}))
	mux.HandleFunc("/api/open-game", guard(func(w http.ResponseWriter, r *http.Request) {
		b, err := os.ReadFile(savePath())
		if err != nil {
			fail(w, "File not found: "+savePath())
			return
		}
		res, err := loadBytes(b)
		if err != nil {
			fail(w, err.Error())
			return
		}
		jsonOut(w, 200, res)
	}))
	edited := func(r *http.Request) ([]byte, error) {
		var st State
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&st); err != nil {
			return nil, fmt.Errorf("invalid data")
		}
		if st.Gems > 99999 {
			return nil, fmt.Errorf("gems must be between 0 and 99999")
		}
		for _, wp := range st.Weapons {
			for _, v := range wp.Up {
				if v > 5 {
					return nil, fmt.Errorf("upgrades must be between 0 and 5")
				}
			}
		}
		mu.Lock()
		cur := current
		mu.Unlock()
		if cur == nil {
			return nil, fmt.Errorf("open a save first")
		}
		return applyState(cur, st)
	}
	mux.HandleFunc("/api/save-game", guard(func(w http.ResponseWriter, r *http.Request) {
		out, err := edited(r)
		if err != nil {
			fail(w, err.Error())
			return
		}
		if err := os.MkdirAll(saveDir(), 0755); err != nil {
			fail(w, "Cannot create the save folder: "+err.Error())
			return
		}
		bk, err := backup()
		if err != nil {
			fail(w, "Backup failed, nothing was saved: "+err.Error())
			return
		}
		if err := os.WriteFile(savePath(), out, 0644); err != nil {
			fail(w, "Could not write the save: "+err.Error())
			return
		}
		mu.Lock()
		current = out
		mu.Unlock()
		jsonOut(w, 200, map[string]string{"path": savePath(), "backup": bk})
	}))
	mux.HandleFunc("/api/download", guard(func(w http.ResponseWriter, r *http.Request) {
		out, err := edited(r)
		if err != nil {
			fail(w, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="AUTOSAVE0.sav"`)
		w.Write(out)
	}))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Println("Could not start the program:", err)
		fmt.Scanln()
		return
	}
	url := "http://" + ln.Addr().String() + "/"
	fmt.Println("SOTD Save Tool - Shadows of the Damned: Hella Remastered")
	fmt.Println()
	fmt.Println("Your browser will open. If it does not, go to:")
	fmt.Println("  " + url)
	fmt.Println()
	fmt.Println("To exit, close this window.")
	go func() {
		time.Sleep(400 * time.Millisecond)
		if runtime.GOOS == "windows" {
			exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
		}
	}()
	http.Serve(ln, mux)
}
