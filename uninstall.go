//go:build windows

package main

// Desinstalador: lista los programas instalados, lanza su desinstalador oficial
// y después busca restos (carpetas y claves de registro) para eliminarlos.

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

type program struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Publisher string `json:"publisher"`
	Version   string `json:"version"`
	Date      string `json:"date"`
	Size      uint64 `json:"size"`
	Location  string `json:"location"`
	Uninstall string `json:"uninstall"`
	Quiet     string `json:"quiet,omitempty"`
	Icon      string `json:"icon,omitempty"`
	Scope     string `json:"scope"`
	Arch      string `json:"arch"`
	MSI       bool   `json:"msi"`
	regRoot   uintptr
	regPath   string
	regView   uint32
}

type uninstSource struct {
	root  uintptr
	path  string
	view  uint32
	scope string
	arch  string
}

var uninstSources = []uninstSource{
	{HKLM, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`, KEY_WOW64_64KEY, "Todos los usuarios", "64 bits"},
	{HKLM, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`, KEY_WOW64_32KEY, "Todos los usuarios", "32 bits"},
	{HKCU, `Software\Microsoft\Windows\CurrentVersion\Uninstall`, 0, "Usuario", ""},
}

var versionTail = regexp.MustCompile(`(?i)[\s\-_(]*(v?\d+(\.\d+)+.*|\(x64\)|\(x86\)|\(64-bit\)|\(32-bit\)|x64|x86)$`)

func listPrograms() []program {
	var out []program
	seen := map[string]bool{}
	for si, s := range uninstSources {
		k, err := regOpen(s.root, s.path, KEY_READ|s.view)
		if err != nil {
			continue
		}
		for _, sub := range k.Subkeys() {
			sk, err := regOpen(s.root, s.path+`\`+sub, KEY_READ|s.view)
			if err != nil {
				continue
			}
			p := program{ID: itoa(int64(si)) + "|" + sub, Name: strings.TrimSpace(sk.Str("DisplayName")), Publisher: sk.Str("Publisher"), Version: sk.Str("DisplayVersion"),
				Location: strings.Trim(sk.Str("InstallLocation"), `" `), Uninstall: sk.Str("UninstallString"), Quiet: sk.Str("QuietUninstallString"), Icon: sk.Str("DisplayIcon"),
				Scope: s.scope, Arch: s.arch, regRoot: s.root, regPath: s.path + `\` + sub, regView: s.view}
			sc, _ := sk.Uint("SystemComponent")
			wi, _ := sk.Uint("WindowsInstaller")
			p.MSI = wi == 1 || strings.Contains(strings.ToLower(p.Uninstall), "msiexec")
			if v, ok := sk.Uint("EstimatedSize"); ok {
				p.Size = v * 1024
			}
			d := sk.Str("InstallDate")
			if len(d) == 8 {
				p.Date = d[6:8] + "/" + d[4:6] + "/" + d[0:4]
			}
			parent := sk.Str("ParentKeyName")
			rel := strings.ToLower(sk.Str("ReleaseType"))
			sk.Close()
			if p.Name == "" || sc == 1 || parent != "" || strings.Contains(rel, "update") || strings.Contains(rel, "hotfix") || p.Uninstall == "" {
				continue
			}
			key := strings.ToLower(p.Name + "|" + p.Version)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, p)
		}
		k.Close()
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

func findProgram(id string) (program, bool) {
	for _, p := range listPrograms() {
		if p.ID == id {
			return p, true
		}
	}
	return program{}, false
}

// ---------- ejecución del desinstalador ----------

type shellExecInfo struct {
	Size      uint32
	Mask      uint32
	Hwnd      uintptr
	Verb      *uint16
	File      *uint16
	Params    *uint16
	Dir       *uint16
	Show      int32
	InstApp   uintptr
	IDList    uintptr
	Class     *uint16
	HkeyClass uintptr
	HotKey    uint32
	Icon      uintptr
	Process   syscall.Handle
}

var pShellExecuteExW = shell32.NewProc("ShellExecuteExW")

func shellRun(file, params string) (syscall.Handle, uint32, error) {
	si := shellExecInfo{Mask: 0x40 | 0x100, Verb: u16("open"), File: u16(file), Show: 1} // NOCLOSEPROCESS | FLAG_NO_UI
	if params != "" {
		si.Params = u16(params)
	}
	si.Size = uint32(unsafe.Sizeof(si))
	if r, _, e := pShellExecuteExW.Call(uintptr(unsafe.Pointer(&si))); r == 0 {
		return 0, 0, e
	}
	var pid uint32
	if si.Process != 0 {
		r, _, _ := kernel32.NewProc("GetProcessId").Call(uintptr(si.Process))
		pid = uint32(r)
	}
	return si.Process, pid, nil
}

type uninstJob struct {
	State   string `json:"state"` // running | done | error
	Message string `json:"message,omitempty"`
	Removed bool   `json:"removed"`
	prog    program
	started time.Time
}

var (
	ujMu  sync.Mutex
	ujobs = map[string]*uninstJob{}
)

func startUninstall(id string) error {
	p, ok := findProgram(id)
	if !ok {
		return errors.New("Programa no encontrado.")
	}
	cmd := p.Uninstall
	exe, args := exeFromCommand(cmd)
	if strings.EqualFold(filepath.Base(exe), "msiexec.exe") || strings.EqualFold(exe, "msiexec") {
		args = regexp.MustCompile(`(?i)/I\s*\{`).ReplaceAllString(args, "/X{")
		if !filepath.IsAbs(exe) {
			exe = filepath.Join(os.Getenv("SystemRoot"), "System32", "msiexec.exe")
		}
	}
	if exe == "" {
		return errors.New("El programa no indica cómo desinstalarse.")
	}
	h, pid, err := shellRun(exe, args)
	if err != nil {
		if errors.Is(err, syscall.Errno(1223)) {
			return errors.New("Has cancelado la solicitud de permisos.")
		}
		return errors.New("No se pudo iniciar el desinstalador: " + err.Error())
	}
	job := &uninstJob{State: "running", prog: p, started: time.Now()}
	ujMu.Lock()
	ujobs[id] = job
	ujMu.Unlock()
	go func() {
		if h != 0 {
			syscall.WaitForSingleObject(h, syscall.INFINITE)
			syscall.CloseHandle(h)
		}
		// muchos desinstaladores se copian a TEMP y continúan en otro proceso: se esperan sus descendientes
		tmp := strings.ToLower(os.Getenv("TEMP"))
		for i := 0; i < 3600; i++ {
			procs := snapshotProcesses()
			kids := descendants(procs, pid)
			alive := false
			for _, pr := range procs {
				lp := strings.ToLower(pr.Path)
				startedAfter := pr.Create > uint64(job.started.UnixNano()/100+116444736000000000)
				if (kids[pr.PID] && pr.PID != pid) || (startedAfter && tmp != "" && strings.HasPrefix(lp, tmp) && (strings.Contains(lp, "un") || strings.Contains(lp, "au_"))) {
					alive = true
					break
				}
			}
			if !alive {
				break
			}
			time.Sleep(time.Second)
		}
		time.Sleep(800 * time.Millisecond)
		ujMu.Lock()
		job.State = "done"
		job.Removed = !regKeyExists(p.regRoot, p.regPath, p.regView)
		ujMu.Unlock()
	}()
	return nil
}

func uninstallStatus(id string) *uninstJob {
	ujMu.Lock()
	defer ujMu.Unlock()
	j := ujobs[id]
	if j == nil {
		return &uninstJob{State: "none"}
	}
	c := *j
	return &c
}

// ---------- restos ----------

type leftover struct {
	Kind  string `json:"kind"` // folder | registry
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	Label string `json:"label"`
}

func cleanName(s string) string {
	s = strings.TrimSpace(versionTail.ReplaceAllString(strings.TrimSpace(s), ""))
	return strings.TrimRight(s, " -_.")
}

func dirSize(p string) int64 {
	var n int64
	filepath.WalkDir(p, func(_ string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if i, e := d.Info(); e == nil {
				n += i.Size()
			}
		}
		return nil
	})
	return n
}

func findLeftovers(id string) []leftover {
	ujMu.Lock()
	j := ujobs[id]
	ujMu.Unlock()
	var p program
	if j != nil {
		p = j.prog
	} else if pp, ok := findProgram(id); ok {
		p = pp
	} else {
		return nil
	}
	names := map[string]bool{}
	for _, n := range []string{p.Name, cleanName(p.Name)} {
		n = strings.TrimSpace(n)
		if len(n) >= 3 {
			names[strings.ToLower(n)] = true
		}
	}
	pub := cleanName(strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(p.Publisher), "."), ", Inc"))
	for _, suf := range []string{" Inc", " LLC", " Ltd", " Corporation", " Corp", " GmbH", " S.L.", " S.A.", " Co."} {
		pub = strings.TrimSuffix(pub, suf)
	}
	var out []leftover
	seen := map[string]bool{}
	addDir := func(path, label string) {
		path = normPath(path)
		k := strings.ToLower(path)
		if path == "" || seen[k] || checkSafePath(path) != nil {
			return
		}
		if st, err := os.Stat(path); err == nil && st.IsDir() {
			seen[k] = true
			out = append(out, leftover{"folder", path, dirSize(path), label})
		}
	}
	if p.Location != "" && len(p.Location) > 3 {
		addDir(p.Location, "Carpeta de instalación")
	}
	roots := []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramData"), os.Getenv("APPDATA"), os.Getenv("LOCALAPPDATA"), filepath.Join(filepath.Dir(os.Getenv("APPDATA")), "LocalLow")}
	for _, r := range roots {
		if r == "" {
			continue
		}
		entries, _ := os.ReadDir(r)
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			ln := strings.ToLower(e.Name())
			if names[ln] {
				addDir(filepath.Join(r, e.Name()), "Datos del programa")
			}
			if pub != "" && len(pub) >= 3 && strings.EqualFold(e.Name(), pub) {
				sub, _ := os.ReadDir(filepath.Join(r, e.Name()))
				for _, s := range sub {
					if s.IsDir() && names[strings.ToLower(s.Name())] {
						addDir(filepath.Join(r, e.Name(), s.Name()), "Datos del programa")
					}
				}
			}
		}
	}
	// registro
	for _, rt := range []struct {
		root uintptr
		base string
	}{{HKCU, "Software"}, {HKLM, "SOFTWARE"}, {HKLM, `SOFTWARE\WOW6432Node`}} {
		k, err := regOpen(rt.root, rt.base, KEY_READ|KEY_WOW64_64KEY)
		if err != nil {
			continue
		}
		for _, s := range k.Subkeys() {
			ls := strings.ToLower(s)
			if names[ls] {
				out = append(out, leftover{"registry", rootName(rt.root) + `\` + rt.base + `\` + s, 0, "Configuración en el registro"})
			}
			if pub != "" && len(pub) >= 3 && strings.EqualFold(s, pub) {
				if pk, err := regOpen(rt.root, rt.base+`\`+s, KEY_READ|KEY_WOW64_64KEY); err == nil {
					for _, ss := range pk.Subkeys() {
						if names[strings.ToLower(ss)] {
							out = append(out, leftover{"registry", rootName(rt.root) + `\` + rt.base + `\` + s + `\` + ss, 0, "Configuración en el registro"})
						}
					}
					pk.Close()
				}
			}
		}
		k.Close()
	}
	// si el programa sigue registrado tras el desinstalador, solo se ofrecen restos cuando la
	// entrada está rota (su desinstalador ya no existe): si no, probablemente se canceló
	if regKeyExists(p.regRoot, p.regPath, p.regView) {
		uexe, _ := exeFromCommand(p.Uninstall)
		broken := uexe != "" && strings.ContainsAny(uexe, `\/`)
		if broken {
			_, err := os.Stat(uexe)
			broken = err != nil
		}
		if !broken {
			return nil
		}
		rp := p.regPath
		if p.regRoot == HKLM && p.regView == KEY_WOW64_32KEY {
			rp = strings.Replace(rp, `SOFTWARE\`, `SOFTWARE\WOW6432Node\`, 1)
		}
		out = append(out, leftover{"registry", rootName(p.regRoot) + `\` + rp, 0, "Entrada de desinstalación huérfana"})
	}
	return out
}

func deleteLeftovers(items []leftover) (int, []string) {
	n := 0
	var errs []string
	for _, it := range items {
		switch it.Kind {
		case "folder":
			p := normPath(it.Path)
			if err := checkSafePath(p); err != nil {
				errs = append(errs, it.Path+": "+err.Error())
				continue
			}
			if recyclePath(p) == nil {
				n++
			} else if os.RemoveAll(p) == nil {
				n++
			} else {
				errs = append(errs, it.Path)
			}
		case "registry":
			var root uintptr
			rest := ""
			switch {
			case strings.HasPrefix(it.Path, "HKEY_CURRENT_USER\\"):
				root, rest = HKCU, strings.TrimPrefix(it.Path, "HKEY_CURRENT_USER\\")
			case strings.HasPrefix(it.Path, "HKEY_LOCAL_MACHINE\\"):
				root, rest = HKLM, strings.TrimPrefix(it.Path, "HKEY_LOCAL_MACHINE\\")
			}
			// nunca borrar claves de primer nivel (Software, SOFTWARE\Microsoft…)
			parts := strings.Split(rest, `\`)
			orphan := strings.Contains(strings.ToLower(rest), `\currentversion\uninstall\`) && len(parts) >= 6
			if root == 0 || !orphan && (len(parts) < 2 || strings.EqualFold(parts[1], "Microsoft") || strings.EqualFold(parts[1], "Classes") || strings.EqualFold(parts[1], "Policies") || (strings.EqualFold(parts[1], "WOW6432Node") && (len(parts) < 3 || strings.EqualFold(parts[2], "Microsoft") || strings.EqualFold(parts[2], "Classes")))) {
				errs = append(errs, it.Path+": protegido")
				continue
			}
			if root == HKLM && !adminCached {
				errs = append(errs, it.Path+": requiere administrador")
				continue
			}
			if err := regDeleteTree(root, rest, KEY_WOW64_64KEY); err == nil {
				n++
			} else {
				errs = append(errs, it.Path)
			}
		}
	}
	return n, errs
}
