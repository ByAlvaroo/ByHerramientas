//go:build windows

package main

// Actualizaciones automáticas desde GitHub Releases.
// El .exe nuevo se descarga junto al actual, se comprueba y se sustituye sin cerrar la ventana.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const updateRepo = "ByAlvaroo/Vantix"
const assetName = "Vantix.exe"

type releaseInfo struct {
	Available bool   `json:"available"`
	Version   string `json:"version"`
	Current   string `json:"current"`
	Notes     string `json:"notes"`
	URL       string `json:"url"`
	Page      string `json:"page"`
	Size      int64  `json:"size"`
	Digest    string `json:"-"`
	Error     string `json:"error,omitempty"`
	Checked   int64  `json:"checked"`
}

var (
	updMu    sync.Mutex
	updLast  *releaseInfo
	updState = map[string]any{"state": "idle"}
)

func verParts(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	var out []int
	for _, p := range strings.Split(v, ".") {
		n, _ := strconv.Atoi(strings.TrimFunc(p, func(r rune) bool { return r < '0' || r > '9' }))
		out = append(out, n)
	}
	return out
}

func newer(a, b string) bool { // ¿a > b?
	x, y := verParts(a), verParts(b)
	for i := 0; i < len(x) || i < len(y); i++ {
		var p, q int
		if i < len(x) {
			p = x[i]
		}
		if i < len(y) {
			q = y[i]
		}
		if p != q {
			return p > q
		}
	}
	return false
}

func checkUpdate() *releaseInfo {
	info := &releaseInfo{Current: appVersion, Checked: time.Now().Unix()}
	c := http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("GET", "https://api.github.com/repos/"+updateRepo+"/releases/latest", nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Vantix-Utilities/"+appVersion)
	r, err := c.Do(req)
	if err != nil {
		info.Error = "Sin conexión con GitHub."
		return info
	}
	defer r.Body.Close()
	if r.StatusCode == 404 {
		info.Error = "Todavía no hay versiones publicadas."
		return info
	}
	if r.StatusCode != 200 {
		info.Error = fmt.Sprintf("GitHub respondió %d.", r.StatusCode)
		return info
	}
	var rel struct {
		Tag    string `json:"tag_name"`
		Body   string `json:"body"`
		Page   string `json:"html_url"`
		Assets []struct {
			Name   string `json:"name"`
			URL    string `json:"browser_download_url"`
			Size   int64  `json:"size"`
			Digest string `json:"digest"`
		} `json:"assets"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&rel) != nil {
		info.Error = "Respuesta de GitHub no válida."
		return info
	}
	info.Version = strings.TrimPrefix(rel.Tag, "v")
	info.Notes, info.Page = rel.Body, rel.Page
	for _, a := range rel.Assets {
		if strings.EqualFold(a.Name, assetName) {
			info.URL, info.Size, info.Digest = a.URL, a.Size, a.Digest
		}
	}
	info.Available = info.URL != "" && newer(info.Version, appVersion)
	updMu.Lock()
	updLast = info
	updMu.Unlock()
	return info
}

func setUpd(k string, v any) {
	updMu.Lock()
	updState[k] = v
	updMu.Unlock()
}

func updateStatus() map[string]any {
	updMu.Lock()
	defer updMu.Unlock()
	m := map[string]any{}
	for k, v := range updState {
		m[k] = v
	}
	if updLast != nil {
		m["release"] = updLast
	}
	return m
}

type progressWriter struct {
	n     int64
	total int64
}

func (p *progressWriter) Write(b []byte) (int, error) {
	p.n += int64(len(b))
	setUpd("done", p.n)
	return len(b), nil
}

// installUpdate descarga la versión nueva y reinicia la aplicación con ella.
func installUpdate() error {
	updMu.Lock()
	info := updLast
	busy := updState["state"] == "downloading"
	updMu.Unlock()
	if busy {
		return errors.New("Ya se está descargando.")
	}
	if info == nil || !info.Available {
		info = checkUpdate()
		if !info.Available {
			return errors.New("No hay ninguna actualización disponible.")
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	dir := filepath.Dir(exe)
	tmp := filepath.Join(dir, "Vantix.new.exe")
	f, err := os.Create(tmp)
	if err != nil {
		return errors.New("No se puede escribir en la carpeta de la aplicación. Muévela a otra carpeta (por ejemplo, Documentos) o ejecútala como administrador.")
	}
	setUpd("state", "downloading")
	setUpd("total", info.Size)
	setUpd("done", int64(0))
	go func() {
		fail := func(msg string) {
			f.Close()
			os.Remove(tmp)
			setUpd("state", "error")
			setUpd("error", msg)
		}
		c := http.Client{Timeout: 10 * time.Minute}
		r, err := c.Get(info.URL)
		if err != nil || r.StatusCode != 200 {
			fail("No se pudo descargar la actualización.")
			return
		}
		defer r.Body.Close()
		h := sha256.New()
		pw := &progressWriter{total: info.Size}
		if _, err := io.Copy(io.MultiWriter(f, h, pw), io.LimitReader(r.Body, 200<<20)); err != nil {
			fail("La descarga se interrumpió.")
			return
		}
		f.Close()
		sum := hex.EncodeToString(h.Sum(nil))
		if d := strings.TrimPrefix(info.Digest, "sha256:"); d != "" && !strings.EqualFold(d, sum) {
			fail("El archivo descargado no coincide con el publicado (suma de comprobación). Se ha descartado.")
			return
		}
		if b, err := os.ReadFile(tmp); err != nil || len(b) < 2 || b[0] != 'M' || b[1] != 'Z' || (info.Size > 0 && int64(len(b)) != info.Size) {
			fail("El archivo descargado no es válido.")
			return
		}
		old := filepath.Join(dir, "Vantix.old.exe")
		os.Remove(old)
		if err := os.Rename(exe, old); err != nil {
			fail("No se pudo sustituir el ejecutable.")
			return
		}
		if err := os.Rename(tmp, exe); err != nil {
			os.Rename(old, exe)
			fail("No se pudo sustituir el ejecutable.")
			return
		}
		setUpd("state", "restarting")
		args := fmt.Sprintf("--handoff --port %d --token %s --edge %d", port, token, edgePID)
		if err := shellExecute("open", exe, args); err != nil {
			setUpd("state", "error")
			setUpd("error", "Actualizado. Cierra y vuelve a abrir la aplicación.")
			return
		}
		time.Sleep(300 * time.Millisecond)
		os.Exit(0)
	}()
	return nil
}

// cleanupOldExe borra la copia anterior que deja una actualización.
func cleanupOldExe() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	old := filepath.Join(filepath.Dir(exe), "Vantix.old.exe")
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(old); err != nil {
			return
		}
		if os.Remove(old) == nil {
			return
		}
		time.Sleep(time.Second)
	}
}
