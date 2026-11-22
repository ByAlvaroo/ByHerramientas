//go:build windows

package main

// Programas de inicio: claves Run del registro, carpetas de Inicio y apps de la Tienda.
// Se activan/desactivan igual que el Administrador de tareas (claves StartupApproved).

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

type startupItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Command   string `json:"command"`
	Exe       string `json:"exe"`
	Publisher string `json:"publisher"`
	Desc      string `json:"desc"`
	Source    string `json:"source"`
	Scope     string `json:"scope"` // Usuario | Todos los usuarios
	Enabled   bool   `json:"enabled"`
	Admin     bool   `json:"admin"`
	Missing   bool   `json:"missing"`
	Running   bool   `json:"running"`
	Size      int64  `json:"size"`
}

const approvedBase = `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\`

type runSource struct {
	root     uintptr
	path     string
	view     uint32
	approved string
	label    string
	scope    string
}

var runSources = []runSource{
	{HKCU, `Software\Microsoft\Windows\CurrentVersion\Run`, 0, "Run", "Registro", "Usuario"},
	{HKLM, `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`, KEY_WOW64_64KEY, "Run", "Registro", "Todos los usuarios"},
	{HKLM, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Run`, KEY_WOW64_64KEY, "Run32", "Registro (32 bits)", "Todos los usuarios"},
}

func startupFolders() []runSource {
	return []runSource{
		{HKCU, filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Start Menu\Programs\Startup`), 0, "StartupFolder", "Carpeta Inicio", "Usuario"},
		{HKLM, filepath.Join(os.Getenv("ProgramData"), `Microsoft\Windows\Start Menu\Programs\StartUp`), 0, "StartupFolder", "Carpeta Inicio", "Todos los usuarios"},
	}
}

// exeFromCommand extrae la ruta del ejecutable de una línea de comandos.
func exeFromCommand(cmd string) (string, string) {
	cmd = strings.TrimSpace(expandEnv(cmd))
	if strings.HasPrefix(cmd, `"`) {
		if i := strings.Index(cmd[1:], `"`); i >= 0 {
			return cmd[1 : i+1], strings.TrimSpace(cmd[i+2:])
		}
	}
	l := strings.ToLower(cmd)
	for _, ext := range []string{".exe", ".com", ".bat", ".cmd", ".lnk"} {
		if i := strings.Index(l, ext); i >= 0 {
			e := i + len(ext)
			if e == len(cmd) || cmd[e] == ' ' {
				return cmd[:e], strings.TrimSpace(cmd[e:])
			}
		}
	}
	f := strings.Fields(cmd)
	if len(f) == 0 {
		return "", ""
	}
	return f[0], strings.TrimSpace(strings.TrimPrefix(cmd, f[0]))
}

// lnkTarget lee la ruta de destino de un acceso directo (.lnk) sin usar COM.
func lnkTarget(p string) string {
	b, err := os.ReadFile(p)
	if err != nil || len(b) < 0x4C || binary.LittleEndian.Uint32(b) != 0x4C {
		return ""
	}
	flags := binary.LittleEndian.Uint32(b[0x14:])
	off := 0x4C
	if flags&1 != 0 && off+2 <= len(b) {
		off += 2 + int(binary.LittleEndian.Uint16(b[off:]))
	}
	if flags&2 == 0 || off+28 > len(b) {
		return ""
	}
	li := b[off:]
	hdr := binary.LittleEndian.Uint32(li[4:])
	if hdr >= 0x24 && len(li) >= 0x20 {
		if uo := int(binary.LittleEndian.Uint32(li[0x1C:])); uo > 0 && uo < len(li) {
			return utf16Bytes(li[uo:])
		}
	}
	lo := int(binary.LittleEndian.Uint32(li[16:]))
	if lo > 0 && lo < len(li) {
		e := lo
		for e < len(li) && li[e] != 0 {
			e++
		}
		return string(li[lo:e])
	}
	return ""
}

func approvedState(root uintptr, sub, name string) (bool, bool) {
	k, err := regOpen(root, approvedBase+sub, KEY_READ|KEY_WOW64_64KEY)
	if err != nil {
		return true, false
	}
	defer k.Close()
	d, _, err := k.raw(name)
	if err != nil || len(d) == 0 {
		return true, false
	}
	return d[0]&1 == 0, true
}

func listStartup() []startupItem {
	var out []startupItem
	running := map[string]bool{}
	for _, p := range snapshotProcesses() {
		if p.Path != "" {
			running[strings.ToLower(p.Path)] = true
		}
	}
	fill := func(it *startupItem) {
		if it.Exe != "" {
			if st, err := os.Stat(it.Exe); err != nil {
				if !strings.ContainsAny(it.Exe, `\/`) {
					it.Missing = false // comando del sistema (p. ej. «rundll32»)
				} else {
					it.Missing = true
				}
			} else {
				it.Size = st.Size()
				vi := cachedVersion(it.Exe)
				it.Publisher, it.Desc = vi.Company, vi.Desc
			}
			it.Running = running[strings.ToLower(it.Exe)]
		}
	}
	for i, s := range runSources {
		k, err := regOpen(s.root, s.path, KEY_READ|s.view)
		if err != nil {
			continue
		}
		for _, v := range k.Values() {
			if v.Type != REG_SZ && v.Type != REG_EXPAND_SZ {
				continue
			}
			cmd := v.String()
			exe, _ := exeFromCommand(cmd)
			it := startupItem{ID: "run" + itoa(int64(i)) + "|" + v.Name, Name: v.Name, Command: cmd, Exe: exe, Source: s.label, Scope: s.scope, Admin: s.root == HKLM}
			it.Enabled, _ = approvedState(s.root, s.approved, v.Name)
			fill(&it)
			out = append(out, it)
		}
		k.Close()
	}
	for i, s := range startupFolders() {
		entries, _ := os.ReadDir(s.path)
		for _, e := range entries {
			if e.IsDir() || strings.EqualFold(e.Name(), "desktop.ini") {
				continue
			}
			full := filepath.Join(s.path, e.Name())
			target := full
			if strings.EqualFold(filepath.Ext(full), ".lnk") {
				if t := lnkTarget(full); t != "" {
					target = t
				}
			}
			it := startupItem{ID: "dir" + itoa(int64(i)) + "|" + e.Name(), Name: strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())), Command: full, Exe: target, Source: s.label, Scope: s.scope, Admin: s.root == HKLM}
			it.Enabled, _ = approvedState(s.root, s.approved, e.Name())
			fill(&it)
			out = append(out, it)
		}
	}
	// apps de la Tienda (StartupTask)
	base := `Software\Classes\Local Settings\Software\Microsoft\Windows\CurrentVersion\AppModel\SystemAppData`
	if k, err := regOpen(HKCU, base, KEY_READ); err == nil {
		for _, pkg := range k.Subkeys() {
			pk, err := regOpen(HKCU, base+`\`+pkg, KEY_READ)
			if err != nil {
				continue
			}
			for _, task := range pk.Subkeys() {
				tk, err := regOpen(HKCU, base+`\`+pkg+`\`+task, KEY_READ)
				if err != nil {
					continue
				}
				if st, ok := tk.Uint("State"); ok {
					name := pkg
					if i := strings.Index(name, "_"); i > 0 {
						name = name[:i]
					}
					if j := strings.LastIndex(name, "."); j > 0 && j < len(name)-1 {
						name = name[j+1:]
					}
					out = append(out, startupItem{ID: "app|" + pkg + `\` + task, Name: name, Command: pkg, Source: "Aplicación de la Tienda", Scope: "Usuario", Enabled: st == 2 || st == 4, Publisher: storePublisher(pkg)})
				}
				tk.Close()
			}
			pk.Close()
		}
		k.Close()
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Enabled != out[j].Enabled {
			return out[i].Enabled
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func storePublisher(pkg string) string {
	l := strings.ToLower(pkg)
	if strings.HasPrefix(l, "microsoft") || strings.HasPrefix(l, "mstea") {
		return "Microsoft Corporation"
	}
	if i := strings.Index(pkg, "."); i > 0 {
		return pkg[:i]
	}
	return ""
}

func setStartup(id string, enable bool) error {
	kind, rest, _ := strings.Cut(id, "|")
	if kind == "app" {
		k, err := regOpen(HKCU, `Software\Classes\Local Settings\Software\Microsoft\Windows\CurrentVersion\AppModel\SystemAppData\`+rest, KEY_READ|KEY_WRITE)
		if err != nil {
			return err
		}
		defer k.Close()
		v := uint32(1)
		if enable {
			v = 2
		}
		return k.SetDword("State", v)
	}
	var s runSource
	if len(kind) != 4 {
		return errors.New("elemento no válido")
	}
	switch {
	case strings.HasPrefix(kind, "run"):
		i := int(kind[3] - '0')
		if i < 0 || i >= len(runSources) {
			return errors.New("elemento no válido")
		}
		s = runSources[i]
	case strings.HasPrefix(kind, "dir"):
		i := int(kind[3] - '0')
		f := startupFolders()
		if i < 0 || i >= len(f) {
			return errors.New("elemento no válido")
		}
		s = f[i]
	default:
		return errors.New("elemento no válido")
	}
	if s.root == HKLM && !adminCached {
		return errors.New("Este programa se inicia para todos los usuarios: hace falta ejecutar como administrador.")
	}
	k, err := regCreate(s.root, approvedBase+s.approved, KEY_READ|KEY_WRITE|KEY_WOW64_64KEY)
	if err != nil {
		return err
	}
	defer k.Close()
	data := make([]byte, 12)
	if enable {
		data[0] = 2
	} else {
		data[0] = 3
		var ft syscall.Filetime = syscall.NsecToFiletime(time.Now().UnixNano())
		binary.LittleEndian.PutUint32(data[4:], ft.LowDateTime)
		binary.LittleEndian.PutUint32(data[8:], ft.HighDateTime)
	}
	return k.SetBinary(rest, data)
}

func deleteStartup(id string) error {
	kind, rest, _ := strings.Cut(id, "|")
	if (strings.HasPrefix(kind, "run") || strings.HasPrefix(kind, "dir")) && (len(kind) != 4 || kind[3] < '0' || kind[3] > '2') {
		return errors.New("elemento no válido")
	}
	if strings.HasPrefix(kind, "dir") && int(kind[3]-'0') >= len(startupFolders()) {
		return errors.New("elemento no válido")
	}
	switch {
	case strings.HasPrefix(kind, "run"):
		s := runSources[int(kind[3]-'0')]
		if s.root == HKLM && !adminCached {
			return errors.New("Hace falta ejecutar como administrador.")
		}
		k, err := regOpen(s.root, s.path, KEY_READ|KEY_WRITE|s.view)
		if err != nil {
			return err
		}
		defer k.Close()
		if err := k.DeleteValue(rest); err != nil {
			return err
		}
		if ak, err := regOpen(s.root, approvedBase+s.approved, KEY_READ|KEY_WRITE|KEY_WOW64_64KEY); err == nil {
			ak.DeleteValue(rest)
			ak.Close()
		}
		return nil
	case strings.HasPrefix(kind, "dir"):
		s := startupFolders()[int(kind[3]-'0')]
		if s.root == HKLM && !adminCached {
			return errors.New("Hace falta ejecutar como administrador.")
		}
		if strings.ContainsAny(rest, `\/`) {
			return errors.New("elemento no válido")
		}
		return recyclePath(filepath.Join(s.path, rest))
	}
	return errors.New("Las apps de la Tienda solo se pueden desactivar.")
}
