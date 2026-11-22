//go:build windows

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// ---------- Instantánea de procesos ----------

type rawProc struct {
	PID     uint32
	PPID    uint32
	Name    string
	Path    string
	Session uint32
	WS      uint64 // conjunto de trabajo (bytes)
	Private uint64
	CPUTime uint64 // kernel + user (100 ns)
	Create  uint64 // FILETIME de creación
	Threads uint32
	Access  bool
	CPU     float64 // % calculado entre muestras
}

func snapshotProcesses() []rawProc {
	h, err := syscall.CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer syscall.CloseHandle(h)
	var e syscall.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	var out []rawProc
	for err = syscall.Process32First(h, &e); err == nil; err = syscall.Process32Next(h, &e) {
		p := rawProc{PID: e.ProcessID, PPID: e.ParentProcessID, Name: syscall.UTF16ToString(e.ExeFile[:]), Threads: e.Threads}
		var sid uint32
		if r, _, _ := pProcessIdToSessionId.Call(uintptr(p.PID), uintptr(unsafe.Pointer(&sid))); r != 0 {
			p.Session = sid
		}
		if p.PID != 0 {
			if ph, err := syscall.OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, false, p.PID); err == nil {
				p.Access = true
				var buf [1024]uint16
				n := uint32(len(buf))
				if r, _, _ := pQueryFullProcessImageNameW.Call(uintptr(ph), 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n))); r != 0 {
					p.Path = syscall.UTF16ToString(buf[:n])
				}
				var mc processMemoryCountersEx
				mc.Cb = uint32(unsafe.Sizeof(mc))
				if r, _, _ := pK32GetProcessMemoryInfo.Call(uintptr(ph), uintptr(unsafe.Pointer(&mc)), uintptr(mc.Cb)); r != 0 {
					p.WS = uint64(mc.WorkingSetSize)
					p.Private = uint64(mc.PrivateUsage)
				}
				var c, x, k, u syscall.Filetime
				if syscall.GetProcessTimes(ph, &c, &x, &k, &u) == nil {
					p.Create = ftU64(c)
					p.CPUTime = ftU64(k) + ftU64(u)
				}
				syscall.CloseHandle(ph)
			}
		}
		out = append(out, p)
	}
	return out
}

// sampler mantiene la última muestra para calcular el % de CPU por proceso.
type sampler struct {
	mu      sync.Mutex
	last    time.Time
	prev    map[uint64]uint64 // clave pid<<32^create -> cputime
	cache   []rawProc
	sysPrev [3]uint64
	sysCPU  float64
}

var smp = &sampler{prev: map[uint64]uint64{}}

func procKey(pid uint32, create uint64) uint64 { return uint64(pid)<<40 ^ create }

// get devuelve una muestra reciente (reutiliza la anterior si tiene menos de maxAge).
func (s *sampler) get(maxAge time.Duration) ([]rawProc, float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if s.cache != nil && now.Sub(s.last) < maxAge {
		return s.cache, s.sysCPU
	}
	procs := snapshotProcesses()
	elapsed := now.Sub(s.last).Seconds()
	ncpu := float64(runtime.NumCPU())
	next := make(map[uint64]uint64, len(procs))
	for i := range procs {
		p := &procs[i]
		k := procKey(p.PID, p.Create)
		next[k] = p.CPUTime
		if old, ok := s.prev[k]; ok && elapsed > 0.2 && p.CPUTime >= old {
			p.CPU = float64(p.CPUTime-old) / 1e7 / elapsed / ncpu * 100
			if p.CPU > 100 {
				p.CPU = 100
			}
		}
	}
	if i, k, u, ok := systemTimes(); ok {
		di, dk, du := i-s.sysPrev[0], k-s.sysPrev[1], u-s.sysPrev[2]
		if s.sysPrev[1] != 0 && dk+du > 0 {
			s.sysCPU = float64(dk+du-di) / float64(dk+du) * 100
			if s.sysCPU < 0 {
				s.sysCPU = 0
			}
		}
		s.sysPrev = [3]uint64{i, k, u}
	}
	s.prev = next
	s.last = now
	s.cache = procs
	return procs, s.sysCPU
}

// ---------- Ventanas visibles por proceso ----------

var (
	winMu     sync.Mutex
	winTitles map[uint32]string
	enumCB    = syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		if v, _, _ := pIsWindowVisible.Call(hwnd); v == 0 {
			return 1
		}
		if owner, _, _ := pGetWindow.Call(hwnd, 4); owner != 0 { // GW_OWNER
			return 1
		}
		ex, _, _ := pGetWindowLongPtrW.Call(hwnd, ^uintptr(19)) // GWL_EXSTYLE = -20
		if ex&0x80 != 0 {                                       // WS_EX_TOOLWINDOW
			return 1
		}
		var cloaked uint32
		if pDwmGetWindowAttribute.Find() == nil {
			pDwmGetWindowAttribute.Call(hwnd, 14, uintptr(unsafe.Pointer(&cloaked)), 4)
		}
		if cloaked != 0 {
			return 1
		}
		n, _, _ := pGetWindowTextLengthW.Call(hwnd)
		if n == 0 {
			return 1
		}
		buf := make([]uint16, n+1)
		pGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), n+1)
		var pid uint32
		pGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if _, ok := winTitles[pid]; !ok {
			winTitles[pid] = syscall.UTF16ToString(buf)
		}
		return 1
	})
)

func visibleWindows() map[uint32]string {
	winMu.Lock()
	defer winMu.Unlock()
	winTitles = map[uint32]string{}
	pEnumWindows.Call(enumCB, 0)
	return winTitles
}

// ---------- Caché de descripciones ----------

var (
	viMu    sync.Mutex
	viCache = map[string]verInfo{}
)

func cachedVersion(path string) verInfo {
	if path == "" {
		return verInfo{}
	}
	k := strings.ToLower(path)
	viMu.Lock()
	v, ok := viCache[k]
	viMu.Unlock()
	if ok {
		return v
	}
	v = fileVersionInfo(path)
	viMu.Lock()
	viCache[k] = v
	viMu.Unlock()
	return v
}

// ---------- Clasificación ----------

// Procesos imprescindibles: nunca se finalizan desde la aplicación.
var criticalNames = set(
	"system", "secure system", "registry", "memory compression", "system idle process", "[system process]",
	"smss.exe", "csrss.exe", "wininit.exe", "winlogon.exe", "services.exe", "lsass.exe", "lsaiso.exe",
	"svchost.exe", "fontdrvhost.exe", "dwm.exe", "explorer.exe", "sihost.exe", "ctfmon.exe", "taskhostw.exe",
	"runtimebroker.exe", "audiodg.exe", "spoolsv.exe", "conhost.exe", "dllhost.exe", "wudfhost.exe",
	"searchindexer.exe", "searchhost.exe", "searchapp.exe", "searchprotocolhost.exe", "searchfilterhost.exe",
	"startmenuexperiencehost.exe", "shellexperiencehost.exe", "textinputhost.exe", "applicationframehost.exe",
	"lockapp.exe", "userinit.exe", "dashost.exe", "wmiprvse.exe", "sgrmbroker.exe", "vmmem", "vmmemwsl",
	"securityhealthservice.exe", "securityhealthsystray.exe", "msmpeng.exe", "nissrv.exe", "mpdefendercoreservice.exe",
	"smartscreen.exe", "wlanext.exe", "unsecapp.exe", "aggregatorhost.exe", "backgroundtaskhost.exe",
	"shellhost.exe", "widgetboard.exe", "dataexchangehost.exe", "useroobebroker.exe", "systemsettingsbroker.exe",
	"openconsole.exe", "msedgewebview2.exe", "igfxem.exe", "atiesrxx.exe", "atieclxx.exe", "nvdisplay.container.exe",
)

// Procesos en segundo plano que se pueden cerrar sin afectar al sistema.
var bloat = map[string]string{
	"googleupdate.exe":              "Actualizador de Google. Se vuelve a abrir solo cuando hace falta.",
	"googlecrashhandler.exe":        "Gestor de informes de errores de Google.",
	"googlecrashhandler64.exe":      "Gestor de informes de errores de Google.",
	"microsoftedgeupdate.exe":       "Actualizador de Microsoft Edge.",
	"msedgeupdate.exe":              "Actualizador de Microsoft Edge.",
	"braveupdate.exe":               "Actualizador de Brave.",
	"opera_autoupdate.exe":          "Actualizador de Opera.",
	"dropboxupdate.exe":             "Actualizador de Dropbox.",
	"adobearm.exe":                  "Actualizador de Adobe Acrobat/Reader.",
	"adobeupdateservice.exe":        "Servicio de actualizaciones de Adobe.",
	"adobenotificationclient.exe":   "Notificaciones de Adobe.",
	"adobeipcbroker.exe":            "Comunicación interna de Adobe Creative Cloud.",
	"ccxprocess.exe":                "Proceso en segundo plano de Creative Cloud.",
	"coresync.exe":                  "Sincronización de archivos de Creative Cloud.",
	"acrotray.exe":                  "Icono de bandeja de Adobe Acrobat.",
	"jusched.exe":                   "Programador de actualizaciones de Java.",
	"jucheck.exe":                   "Comprobador de actualizaciones de Java.",
	"phoneexperiencehost.exe":       "Enlace Móvil (Phone Link) en segundo plano.",
	"yourphone.exe":                 "Enlace Móvil (Phone Link) en segundo plano.",
	"yourphoneserver.exe":           "Enlace Móvil (Phone Link) en segundo plano.",
	"gamebar.exe":                   "Xbox Game Bar en segundo plano.",
	"gamebarftserver.exe":           "Servidor de Xbox Game Bar.",
	"gamebarpresencewriter.exe":     "Registro de actividad de Xbox Game Bar.",
	"widgets.exe":                   "Panel de widgets de Windows.",
	"widgetservice.exe":             "Servicio del panel de widgets.",
	"cortana.exe":                   "Cortana.",
	"skypeapp.exe":                  "Skype en segundo plano.",
	"skypebackgroundhost.exe":       "Skype en segundo plano.",
	"hxtsr.exe":                     "Sincronización de la antigua app Correo y Calendario.",
	"msoia.exe":                     "Telemetría de Microsoft Office.",
	"crossdeviceresume.exe":         "Continuar en otro dispositivo (Windows).",
	"crossdeviceservice.exe":        "Servicio entre dispositivos de Windows.",
	"ms-teamsupdate.exe":            "Actualizador de Microsoft Teams.",
	"microsoft.photos.exe":          "Fotos de Windows en segundo plano.",
	"video.ui.exe":                  "Películas y TV en segundo plano.",
	"epicwebhelper.exe":             "Componente web de Epic Games en segundo plano.",
	"eabackgroundservice.exe":       "Servicio en segundo plano de EA app.",
	"origin web helper service.exe": "Servicio en segundo plano de Origin.",
	"nvidia web helper.exe":         "Componente web de NVIDIA GeForce Experience.",
	"hpprintscandoctor.exe":         "Asistente de HP en segundo plano.",
	"copilot.exe":                   "Copilot en segundo plano.",
}

var securityVendors = []string{"microsoft defender", "windows defender", "avast", "avg ", "avira", "eset", "kaspersky", "mcafee", "norton", "symantec", "bitdefender", "malwarebytes", "sophos", "trend micro", "panda security", "f-secure", "webroot", "gen digital"}
var hardwareVendors = []string{"nvidia", "advanced micro devices", "amd ", "intel", "realtek", "logitech", "razer", "corsair", "asustek", "asus", "micro-star", "msi ", "gigabyte", "steelseries", "hyperx", "synaptics", "elan micro", "dolby", "waves", "conexant", "nahimic", "a-volute", "broadcom", "qualcomm", "mediatek", "rivet", "killer", "western digital", "samsung", "wacom", "xp-pen", "ugee", "hanvon", "huion", "cooler master", "nzxt", "glorious", "ajazz", "roccat", "turtle beach", "creative technology", "elgato", "focusrite"}

func set(xs ...string) map[string]bool {
	m := make(map[string]bool, len(xs))
	for _, x := range xs {
		m[x] = true
	}
	return m
}

func containsAny(s string, list []string) bool {
	s = " " + strings.ToLower(s) + " "
	for _, v := range list {
		if strings.Contains(s, v) {
			return true
		}
	}
	return false
}

var winDir = strings.ToLower(os.Getenv("SystemRoot"))

// ---------- Grupos de procesos para la interfaz ----------

type procMember struct {
	PID    uint32  `json:"pid"`
	CT     uint64  `json:"ct"`
	WS     uint64  `json:"ws"`
	CPU    float64 `json:"cpu"`
	Window string  `json:"win,omitempty"`
}

type procGroup struct {
	Key       string       `json:"key"`
	Name      string       `json:"name"`
	Exe       string       `json:"exe"`
	Path      string       `json:"path"`
	Company   string       `json:"company"`
	Count     int          `json:"count"`
	WS        uint64       `json:"ws"`
	CPU       float64      `json:"cpu"`
	Window    string       `json:"window,omitempty"`
	Category  string       `json:"cat"`    // system | security | hardware | app | bloat | background | service | self
	Protected bool         `json:"locked"` // no se puede finalizar desde la app
	Auto      bool         `json:"auto"`   // incluido en la limpieza recomendada
	Reason    string       `json:"reason"`
	Members   []procMember `json:"members"`
}

// protegidos por la aplicación: ella misma y su ventana de Edge
var (
	selfPID     uint32
	edgePID     uint32
	adminCached bool
)

func descendants(procs []rawProc, root uint32) map[uint32]bool {
	out := map[uint32]bool{}
	if root == 0 {
		return out
	}
	children := map[uint32][]uint32{}
	created := map[uint32]uint64{}
	for _, p := range procs {
		children[p.PPID] = append(children[p.PPID], p.PID)
		created[p.PID] = p.Create
	}
	if _, ok := created[root]; !ok {
		return out
	}
	stack := []uint32{root}
	out[root] = true
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, c := range children[n] {
			// el hijo debe ser posterior al padre (evita PIDs reutilizados)
			if !out[c] && created[c] >= created[n] {
				out[c] = true
				stack = append(stack, c)
			}
		}
	}
	return out
}

func buildGroups(cfg Config) ([]procGroup, map[string]any) {
	procs, sysCPU := smp.get(900 * time.Millisecond)
	wins := visibleWindows()
	own := descendants(procs, edgePID)
	own[selfPID] = true

	excl := map[string]bool{}
	for _, x := range cfg.Exclusions {
		excl[strings.ToLower(x)] = true
	}
	always := map[string]bool{}
	for _, x := range cfg.AlwaysClose {
		always[strings.ToLower(x)] = true
	}

	groups := map[string]*procGroup{}
	var order []string
	for _, p := range procs {
		if p.PID == 0 {
			continue
		}
		key := strings.ToLower(p.Path)
		if key == "" {
			key = "name:" + strings.ToLower(p.Name)
		}
		if own[p.PID] {
			key = "self"
		}
		g := groups[key]
		if g == nil {
			g = &procGroup{Key: key, Exe: p.Name, Path: p.Path}
			groups[key] = g
			order = append(order, key)
		}
		m := procMember{PID: p.PID, CT: p.Create, WS: p.WS, CPU: p.CPU}
		if t, ok := wins[p.PID]; ok {
			m.Window = t
			if g.Window == "" {
				g.Window = t
			}
		}
		g.Members = append(g.Members, m)
		g.Count++
		g.WS += p.WS
		g.CPU += p.CPU
		if p.Session == 0 && g.Category == "" {
			g.Category = "service0"
		}
	}

	out := make([]procGroup, 0, len(order))
	for _, k := range order {
		g := groups[k]
		vi := cachedVersion(g.Path)
		g.Company = vi.Company
		g.Name = vi.Desc
		if g.Name == "" || strings.EqualFold(g.Name, g.Exe) {
			g.Name = strings.TrimSuffix(g.Exe, filepath.Ext(g.Exe))
		}
		classify(g, excl, always, cfg)
		sort.Slice(g.Members, func(i, j int) bool { return g.Members[i].WS > g.Members[j].WS })
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].WS > out[j].WS })

	ms := memStatus()
	totals := map[string]any{
		"cpu": sysCPU, "memUsed": ms.TotalPhys - ms.AvailPhys, "memTotal": ms.TotalPhys,
		"processes": len(procs), "admin": isAdmin(),
	}
	return out, totals
}

func classify(g *procGroup, excl, always map[string]bool, cfg Config) {
	exe := strings.ToLower(g.Exe)
	lp := strings.ToLower(g.Path)
	svc := g.Category == "service0"
	g.Category = ""
	switch {
	case g.Key == "self":
		g.Name = "Vantix Utilities"
		g.Category, g.Protected, g.Reason = "self", true, "Esta aplicación y su ventana."
	case criticalNames[exe] || exe == "system" || g.Path == "" && !strings.HasSuffix(exe, ".exe"):
		g.Category, g.Protected, g.Reason = "system", true, "Proceso esencial de Windows. Finalizarlo puede bloquear o reiniciar el equipo."
		if exe == "explorer.exe" {
			g.Reason = "Explorador de Windows (escritorio y barra de tareas). Usa «Reiniciar Explorador» si se queda colgado."
		}
	case containsAny(g.Company, securityVendors) || containsAny(g.Name, []string{"antivirus", "defender"}):
		g.Category, g.Protected, g.Reason = "security", true, "Protección antivirus o de seguridad. No se debe cerrar."
	case winDir != "" && strings.HasPrefix(lp, winDir+`\`) && bloat[exe] == "":
		g.Category, g.Protected, g.Reason = "system", true, "Componente de Windows."
	case svc && bloat[exe] == "":
		g.Category, g.Protected, g.Reason = "service", true, "Servicio del sistema (se ejecuta fuera de tu sesión)."
	case g.Window != "":
		g.Category, g.Reason = "app", "Aplicación abierta con ventana. Ciérrala solo si no la estás usando: perderías lo no guardado."
	case bloat[exe] != "":
		g.Category, g.Reason = "bloat", bloat[exe]+" No es necesario para que el equipo funcione."
	case containsAny(g.Company, hardwareVendors):
		g.Category, g.Reason = "hardware", "Software de controladores o periféricos ("+g.Company+"). Recomendado mantener: controla audio, gráficos, ratón, teclado o iluminación."
	default:
		g.Category, g.Reason = "background", "Programa en segundo plano sin ventana (bandeja del sistema, sincronización, asistentes…). Ciérralo si no lo necesitas."
	}
	if g.Protected {
		return
	}
	if excl[exe] {
		g.Auto = false
		g.Reason = "Excluido por ti: nunca se cierra en la limpieza. " + g.Reason
		return
	}
	switch {
	case always[exe]:
		g.Auto = true
		g.Reason = "Marcado por ti para cerrar siempre en la limpieza. " + g.Reason
	case g.Category == "bloat":
		// un servicio del sistema solo se puede cerrar con permisos de administrador
		g.Auto = !svc || adminCached
	case g.Category == "background" && cfg.IncludeHeavy && g.WS >= uint64(cfg.HeavyMB)<<20:
		g.Auto = true
		g.Reason = "Supera el umbral de memoria que configuraste. " + g.Reason
	}
}

// ---------- Finalizar procesos ----------

type killTarget struct {
	PID uint32 `json:"pid"`
	CT  uint64 `json:"ct"`
}

type killResult struct {
	PID   uint32 `json:"pid"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func isCriticalPID(pid uint32, procs []rawProc) bool {
	for _, p := range procs {
		if p.PID == pid {
			n := strings.ToLower(p.Name)
			return criticalNames[n] && n != "explorer.exe"
		}
	}
	return false
}

// killProcess finaliza un proceso comprobando que el PID sigue siendo el mismo proceso.
func killProcess(t killTarget) killResult {
	res := killResult{PID: t.PID}
	if t.PID == 0 || t.PID == 4 || t.PID == selfPID {
		res.Error = "Proceso protegido"
		return res
	}
	h, err := syscall.OpenProcess(PROCESS_TERMINATE|SYNCHRONIZE|PROCESS_QUERY_LIMITED_INFORMATION, false, t.PID)
	if err != nil {
		if err == syscall.ERROR_ACCESS_DENIED {
			res.Error = "Requiere permisos de administrador"
		} else if err == syscall.Errno(87) {
			res.OK = true // ya no existe
		} else {
			res.Error = err.Error()
		}
		return res
	}
	defer syscall.CloseHandle(h)
	if t.CT != 0 {
		var c, x, k, u syscall.Filetime
		if syscall.GetProcessTimes(h, &c, &x, &k, &u) == nil && ftU64(c) != t.CT {
			res.OK = true // el proceso original ya terminó y el PID se reutilizó
			return res
		}
	}
	if err := syscall.TerminateProcess(h, 1); err != nil {
		if err == syscall.ERROR_ACCESS_DENIED {
			res.Error = "Requiere permisos de administrador"
		} else {
			res.Error = err.Error()
		}
		return res
	}
	syscall.WaitForSingleObject(h, 3000)
	res.OK = true
	return res
}

func killMany(ts []killTarget) []killResult {
	procs, _ := smp.get(0)
	out := make([]killResult, len(ts))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i, t := range ts {
		if isCriticalPID(t.PID, procs) {
			out[i] = killResult{PID: t.PID, Error: "Proceso esencial de Windows: no se finaliza"}
			continue
		}
		wg.Add(1)
		go func(i int, t killTarget) {
			defer wg.Done()
			sem <- struct{}{}
			out[i] = killProcess(t)
			<-sem
		}(i, t)
	}
	wg.Wait()
	smp.mu.Lock()
	smp.cache = nil // fuerza una muestra nueva en la próxima consulta
	smp.mu.Unlock()
	return out
}

func restartExplorer() error {
	for _, p := range snapshotProcesses() {
		if strings.EqualFold(p.Name, "explorer.exe") {
			killProcess(killTarget{PID: p.PID, CT: p.Create})
		}
	}
	// Windows suele relanzarlo solo; si no lo hace en 2 s, se inicia manualmente.
	for i := 0; i < 8; i++ {
		time.Sleep(250 * time.Millisecond)
		for _, p := range snapshotProcesses() {
			if strings.EqualFold(p.Name, "explorer.exe") {
				return nil
			}
		}
	}
	return shellExecute("open", filepath.Join(os.Getenv("SystemRoot"), "explorer.exe"), "")
}

// ---------- Lectura del directorio de trabajo de otro proceso (PEB) ----------

func readMem(h syscall.Handle, addr uintptr, buf unsafe.Pointer, size uintptr) bool {
	var n uintptr
	r, _, _ := pReadProcessMemory.Call(uintptr(h), addr, uintptr(buf), size, uintptr(unsafe.Pointer(&n)))
	return r != 0 && n == size
}

// processCwd devuelve el directorio de trabajo de un proceso de 64 bits (o WOW64) de la sesión.
func processCwd(pid uint32) string {
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		return ""
	}
	h, err := syscall.OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION|PROCESS_VM_READ, false, pid)
	if err != nil {
		return ""
	}
	defer syscall.CloseHandle(h)
	var pbi [6]uintptr // PROCESS_BASIC_INFORMATION
	var rl uint32
	st, _, _ := pNtQueryInformationProcess.Call(uintptr(h), 0, uintptr(unsafe.Pointer(&pbi[0])), unsafe.Sizeof(pbi), uintptr(unsafe.Pointer(&rl)))
	if st != 0 || pbi[1] == 0 {
		return ""
	}
	var params uintptr
	if !readMem(h, pbi[1]+0x20, unsafe.Pointer(&params), 8) || params == 0 {
		return ""
	}
	var us struct {
		Length, Max uint16
		_           uint32
		Buffer      uintptr
	}
	if !readMem(h, params+0x38, unsafe.Pointer(&us), unsafe.Sizeof(us)) || us.Length == 0 || us.Length > 2048 || us.Buffer == 0 {
		return ""
	}
	buf := make([]uint16, us.Length/2)
	if !readMem(h, us.Buffer, unsafe.Pointer(&buf[0]), uintptr(us.Length)) {
		return ""
	}
	return syscall.UTF16ToString(buf)
}
