//go:build windows

// Vantix Utilities — utilidades del sistema para Windows.
// Servidor local (solo 127.0.0.1, protegido con token) + interfaz en una ventana de Edge en modo aplicación.
package main

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const appName = "Vantix Utilities"
const appVersion = "1.0.0"
const defaultPort = 47615

//go:embed ui
var uiFS embed.FS

// ---------- Configuración persistente (%APPDATA%\Vantix\config.json) ----------

type Config struct {
	Exclusions   []string        `json:"exclusions"`
	AlwaysClose  []string        `json:"alwaysClose"`
	ConfirmKill  bool            `json:"confirmKill"`
	IncludeHeavy bool            `json:"includeHeavy"`
	HeavyMB      int             `json:"heavyMB"`
	Refresh      int             `json:"refresh"` // segundos entre actualizaciones
	Search       json.RawMessage `json:"search,omitempty"`
}

var (
	cfgMu sync.Mutex
	cfg   = Config{ConfirmKill: true, HeavyMB: 400, Refresh: 3, Exclusions: []string{}, AlwaysClose: []string{}}
)

func dataDir() string {
	d := filepath.Join(os.Getenv("APPDATA"), "Vantix")
	os.MkdirAll(d, 0755)
	return d
}

func loadConfig() {
	b, err := os.ReadFile(filepath.Join(dataDir(), "config.json"))
	if err != nil {
		return
	}
	cfgMu.Lock()
	json.Unmarshal(b, &cfg)
	if cfg.HeavyMB < 50 {
		cfg.HeavyMB = 400
	}
	if cfg.Refresh < 1 || cfg.Refresh > 30 {
		cfg.Refresh = 3
	}
	if cfg.Exclusions == nil {
		cfg.Exclusions = []string{}
	}
	if cfg.AlwaysClose == nil {
		cfg.AlwaysClose = []string{}
	}
	cfgMu.Unlock()
}

func saveConfig() {
	cfgMu.Lock()
	b, _ := json.MarshalIndent(cfg, "", "  ")
	cfgMu.Unlock()
	tmp := filepath.Join(dataDir(), "config.json.tmp")
	if os.WriteFile(tmp, b, 0644) == nil {
		os.Rename(tmp, filepath.Join(dataDir(), "config.json"))
	}
}

func getConfig() Config {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	return cfg
}

// ---------- Ciclo de vida ----------

var (
	token     string
	port      int
	clients   int64
	lastSeen  atomic.Int64
	elevating atomic.Bool
)

func randToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

type instanceInfo struct {
	Port  int    `json:"port"`
	Token string `json:"token"`
}

func instanceFile() string { return filepath.Join(os.Getenv("LOCALAPPDATA"), "Vantix", "instance.json") }

func main() {
	elevated := flag.Bool("elevated", false, "")
	fPort := flag.Int("port", 0, "")
	fToken := flag.String("token", "", "")
	fEdge := flag.Int("edge", 0, "")
	flag.Parse()

	selfPID = currentPID()
	adminCached = isAdmin()
	loadConfig()
	os.MkdirAll(filepath.Join(os.Getenv("LOCALAPPDATA"), "Vantix"), 0755)

	// ¿Ya hay una instancia abierta? Se abre su ventana y se sale.
	if !*elevated {
		if b, err := os.ReadFile(instanceFile()); err == nil {
			var ii instanceInfo
			if json.Unmarshal(b, &ii) == nil && ii.Port > 0 {
				c := http.Client{Timeout: 700 * time.Millisecond}
				if r, err := c.Get(fmt.Sprintf("http://127.0.0.1:%d/api/ping?t=%s", ii.Port, ii.Token)); err == nil {
					body, _ := io.ReadAll(r.Body)
					r.Body.Close()
					if string(body) == "vantix" {
						openWindow(fmt.Sprintf("http://127.0.0.1:%d/?t=%s", ii.Port, ii.Token))
						return
					}
				}
			}
		}
	}

	token = *fToken
	if token == "" {
		token = randToken()
	}
	var ln net.Listener
	var err error
	want := *fPort
	if want == 0 {
		want = defaultPort
	}
	// Al reiniciar como administrador, la instancia anterior tarda un instante en liberar el puerto.
	for i := 0; i < 60; i++ {
		ln, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", want))
		if err == nil || !*elevated {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if err != nil {
		ln, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			msgBox("No se pudo iniciar " + appName + ": " + err.Error())
			return
		}
	}
	port = ln.Addr().(*net.TCPAddr).Port
	if b, err := json.Marshal(instanceInfo{port, token}); err == nil {
		os.WriteFile(instanceFile(), b, 0600)
	}

	srv := &http.Server{Handler: routes(), ReadHeaderTimeout: 10 * time.Second}
	go srv.Serve(ln)

	url := fmt.Sprintf("http://127.0.0.1:%d/?t=%s", port, token)
	if *elevated && *fEdge > 0 {
		edgePID = uint32(*fEdge) // la ventana anterior sigue abierta y se reconecta sola
	} else {
		edgePID = openWindow(url)
	}
	smp.get(0) // primera muestra para el % de CPU

	// Se cierra solo cuando no queda ninguna ventana conectada.
	lastSeen.Store(time.Now().Unix())
	for {
		time.Sleep(2 * time.Second)
		if atomic.LoadInt64(&clients) > 0 {
			lastSeen.Store(time.Now().Unix())
			continue
		}
		idle := time.Now().Unix() - lastSeen.Load()
		if idle > 12 {
			if b, err := os.ReadFile(instanceFile()); err == nil && strings.Contains(string(b), token) {
				os.Remove(instanceFile())
			}
			os.Exit(0)
		}
	}
}

func msgBox(text string) {
	user32.NewProc("MessageBoxW").Call(0, uintptr(unsafePtr(u16(text))), uintptr(unsafePtr(u16(appName))), 0x10)
}

// openWindow abre la interfaz en Edge (o Chrome) en modo aplicación, con un perfil propio.
// Devuelve el PID del navegador lanzado para protegerlo de la limpieza.
func openWindow(url string) uint32 {
	cands := []string{
		filepath.Join(os.Getenv("ProgramFiles(x86)"), `Microsoft\Edge\Application\msedge.exe`),
		filepath.Join(os.Getenv("ProgramFiles"), `Microsoft\Edge\Application\msedge.exe`),
		filepath.Join(os.Getenv("LOCALAPPDATA"), `Microsoft\Edge\Application\msedge.exe`),
		filepath.Join(os.Getenv("ProgramFiles"), `Google\Chrome\Application\chrome.exe`),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), `Google\Chrome\Application\chrome.exe`),
		filepath.Join(os.Getenv("LOCALAPPDATA"), `Google\Chrome\Application\chrome.exe`),
	}
	profile := filepath.Join(os.Getenv("LOCALAPPDATA"), "Vantix", "WebView")
	for _, c := range cands {
		if _, err := os.Stat(c); err != nil {
			continue
		}
		cmd := exec.Command(c, "--app="+url, "--user-data-dir="+profile, "--window-size=1380,880",
			"--no-first-run", "--no-default-browser-check", "--disable-extensions", "--disable-sync",
			"--disable-background-networking", "--disable-component-update", "--disable-features=Translate,msEdgeSidebarV2,msUndersideButton,EdgeCollections",
			"--disable-default-apps")
		if err := cmd.Start(); err == nil {
			pid := uint32(cmd.Process.Pid)
			cmd.Process.Release()
			return pid
		}
	}
	shellExecute("open", url, "")
	return 0
}

// ---------- HTTP ----------

func routes() http.Handler {
	mux := http.NewServeMux()
	sub, _ := fs.Sub(uiFS, "ui")
	static := http.FileServer(http.FS(sub))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" && r.URL.Query().Get("t") != token {
			http.Error(w, "No autorizado. Abre la aplicación desde su icono.", 403)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		static.ServeHTTP(w, r)
	})
	mux.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("t") == token {
			io.WriteString(w, "vantix")
		}
	})
	api := func(path string, h http.HandlerFunc) {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			t := r.Header.Get("X-Token")
			if t == "" {
				t = r.URL.Query().Get("t")
			}
			if t != token {
				http.Error(w, "no autorizado", 403)
				return
			}
			// bloquea peticiones de otras webs (DNS rebinding / CSRF)
			if h := r.Host; !strings.HasPrefix(h, "127.0.0.1:") && !strings.HasPrefix(h, "localhost:") {
				http.Error(w, "host no permitido", 403)
				return
			}
			h(w, r)
		})
	}

	api("/api/alive", func(w http.ResponseWriter, r *http.Request) {
		fl, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		atomic.AddInt64(&clients, 1)
		defer func() { atomic.AddInt64(&clients, -1); lastSeen.Store(time.Now().Unix()) }()
		fmt.Fprint(w, "event: hi\ndata: 1\n\n")
		if fl != nil {
			fl.Flush()
		}
		t := time.NewTicker(20 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-t.C:
				fmt.Fprint(w, ": k\n\n")
				if fl != nil {
					fl.Flush()
				}
			}
		}
	})

	api("/api/status", func(w http.ResponseWriter, r *http.Request) {
		host, _ := os.Hostname()
		writeJSON(w, map[string]any{"admin": isAdmin(), "version": appVersion, "host": host, "user": os.Getenv("USERNAME")})
	})

	api("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, systemStats())
	})

	api("/api/processes", func(w http.ResponseWriter, r *http.Request) {
		groups, totals := buildGroups(getConfig())
		writeJSON(w, map[string]any{"groups": groups, "totals": totals})
	})

	api("/api/kill", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Targets []killTarget `json:"targets"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		writeJSON(w, map[string]any{"results": killMany(req.Targets)})
	})

	api("/api/explorer/restart", func(w http.ResponseWriter, r *http.Request) {
		if err := restartExplorer(); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})

	api("/api/unlock/analyze", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Path string `json:"path"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		a, err := analyzePath(req.Path)
		if err != nil {
			writeJSON(w, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, a)
	})

	api("/api/unlock/delete", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Path    string       `json:"path"`
			Targets []killTarget `json:"targets"`
			Recycle bool         `json:"recycle"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		writeJSON(w, forceDelete(req.Path, req.Targets, req.Recycle))
	})

	api("/api/unlock/schedule", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Path string `json:"path"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		n, err := scheduleOnReboot(req.Path)
		if err != nil {
			writeJSON(w, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "count": n})
	})

	api("/api/pick", func(w http.ResponseWriter, r *http.Request) {
		folder := r.URL.Query().Get("kind") != "file"
		title := "Selecciona la carpeta"
		if !folder {
			title = "Selecciona el archivo"
		}
		p, err := pickPath(folder, title)
		if err != nil {
			writeJSON(w, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"path": p})
	})

	api("/api/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var c Config
			if !readJSON(w, r, &c) {
				return
			}
			cfgMu.Lock()
			if c.Search == nil {
				c.Search = cfg.Search
			}
			if c.HeavyMB < 50 {
				c.HeavyMB = 50
			}
			if c.Refresh < 1 || c.Refresh > 30 {
				c.Refresh = 3
			}
			if c.Exclusions == nil {
				c.Exclusions = []string{}
			}
			if c.AlwaysClose == nil {
				c.AlwaysClose = []string{}
			}
			cfg = c
			cfgMu.Unlock()
			saveConfig()
		}
		writeJSON(w, getConfig())
	})

	api("/api/elevate", func(w http.ResponseWriter, r *http.Request) {
		if isAdmin() {
			writeJSON(w, map[string]any{"ok": true, "already": true})
			return
		}
		exe, _ := os.Executable()
		args := fmt.Sprintf("--elevated --port %d --token %s --edge %d", port, token, edgePID)
		if err := shellExecute("runas", exe, args); err != nil {
			msg := "No se pudo reiniciar como administrador."
			if errors.Is(err, syscall.Errno(1223)) {
				msg = "Has cancelado la solicitud de permisos."
			}
			writeJSON(w, map[string]any{"error": msg})
			return
		}
		elevating.Store(true)
		writeJSON(w, map[string]any{"ok": true})
		go func() {
			time.Sleep(300 * time.Millisecond)
			os.Exit(0)
		}()
	})

	api("/api/drives", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"drives": listDrives(), "home": os.Getenv("USERPROFILE")})
	})

	api("/api/search", handleSearch)

	api("/api/open", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Path string `json:"path"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		if _, err := os.Lstat(req.Path); err != nil {
			http.Error(w, "ya no existe", 404)
			return
		}
		if err := shellExecute("open", req.Path, ""); err != nil {
			http.Error(w, err.Error(), 500)
		}
	})

	api("/api/reveal", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Path string `json:"path"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		if _, err := os.Lstat(req.Path); err != nil {
			http.Error(w, "ya no existe", 404)
			return
		}
		cmd := exec.Command(filepath.Join(os.Getenv("SystemRoot"), "explorer.exe"))
		cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer.exe /select,"` + req.Path + `"`}
		cmd.Start()
		if cmd.Process != nil {
			cmd.Process.Release()
		}
	})
	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(v)
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.Method != http.MethodPost {
		http.Error(w, "método no permitido", 405)
		return false
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(v); err != nil {
		http.Error(w, "petición no válida", 400)
		return false
	}
	return true
}

// ---------- Resumen del sistema ----------

func systemStats() map[string]any {
	procs, cpu := smp.get(900 * time.Millisecond)
	ms := memStatus()
	var threads uint32
	type top struct {
		Name string  `json:"name"`
		Exe  string  `json:"exe"`
		WS   uint64  `json:"ws"`
		CPU  float64 `json:"cpu"`
		N    int     `json:"n"`
	}
	agg := map[string]*top{}
	own := descendants(procs, edgePID)
	for _, p := range procs {
		threads += p.Threads
		if p.PID == 0 {
			continue
		}
		k := strings.ToLower(p.Name)
		if own[p.PID] || p.PID == selfPID {
			k = "\x00self"
		}
		t := agg[k]
		if t == nil {
			t = &top{Exe: p.Name}
			if k == "\x00self" {
				t.Name, t.Exe = appName, "vantix"
			} else {
				vi := cachedVersion(p.Path)
				t.Name = vi.Desc
				if t.Name == "" {
					t.Name = strings.TrimSuffix(p.Name, filepath.Ext(p.Name))
				}
			}
			agg[k] = t
		}
		t.WS += p.WS
		t.CPU += p.CPU
		t.N++
	}
	list := make([]top, 0, len(agg))
	for _, t := range agg {
		list = append(list, *t)
	}
	byMem := append([]top(nil), list...)
	sortTop(byMem, func(a, b top) bool { return a.WS > b.WS })
	byCPU := append([]top(nil), list...)
	sortTop(byCPU, func(a, b top) bool { return a.CPU > b.CPU })
	if len(byMem) > 6 {
		byMem = byMem[:6]
	}
	if len(byCPU) > 6 {
		byCPU = byCPU[:6]
	}
	var disks []driveInfo
	for _, d := range listDrives() {
		if (d.Kind == "fixed" || d.Kind == "removable") && d.Total > 0 {
			disks = append(disks, d)
		}
	}
	return map[string]any{
		"cpu": cpu, "cores": numCPU(), "memLoad": ms.MemoryLoad, "memTotal": ms.TotalPhys, "memAvail": ms.AvailPhys,
		"commitTotal": ms.TotalPageFile, "commitAvail": ms.AvailPageFile,
		"uptime": tick64() / 1000, "processes": len(procs), "threads": threads,
		"topMem": byMem, "topCPU": byCPU, "disks": disks, "admin": isAdmin(),
	}
}

func sortTop[T any](s []T, less func(a, b T) bool) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && less(s[j], s[j-1]); j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func numCPU() int {
	n, _ := strconv.Atoi(os.Getenv("NUMBER_OF_PROCESSORS"))
	return n
}
