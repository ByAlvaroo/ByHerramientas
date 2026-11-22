//go:build windows

package main

// Eliminación forzada: detecta qué procesos bloquean una carpeta o archivo,
// los finaliza y elimina el contenido.

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// RM_PROCESS_INFO (Restart Manager)
type rmProcessInfo struct {
	PID         uint32
	StartLow    uint32
	StartHigh   uint32
	AppName     [256]uint16
	ServiceName [64]uint16
	AppType     int32
	AppStatus   uint32
	TSSessionID uint32
	Restartable int32
}

// lockingProcesses usa el Administrador de reinicio de Windows para saber qué
// procesos tienen abiertos los archivos indicados.
func lockingProcesses(files []string) map[uint32]string {
	out := map[uint32]string{}
	if pRmStartSession.Find() != nil {
		return out
	}
	const batch = 1500
	for i := 0; i < len(files); i += batch {
		j := i + batch
		if j > len(files) {
			j = len(files)
		}
		var session uint32
		var key [33]uint16
		if r, _, _ := pRmStartSession.Call(uintptr(unsafe.Pointer(&session)), 0, uintptr(unsafe.Pointer(&key[0]))); r != 0 {
			continue
		}
		ptrs := make([]*uint16, 0, j-i)
		for _, f := range files[i:j] {
			ptrs = append(ptrs, u16(longPath(f)))
		}
		r, _, _ := pRmRegisterResources.Call(uintptr(session), uintptr(len(ptrs)), uintptr(unsafe.Pointer(&ptrs[0])), 0, 0, 0, 0)
		if r == 0 {
			var needed, n, reasons uint32
			for tries := 0; tries < 4; tries++ {
				infos := make([]rmProcessInfo, needed+8)
				n = uint32(len(infos))
				r, _, _ = pRmGetList.Call(uintptr(session), uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&n)), uintptr(unsafe.Pointer(&infos[0])), uintptr(unsafe.Pointer(&reasons)))
				if r == 234 { // ERROR_MORE_DATA
					continue
				}
				if r == 0 {
					for _, in := range infos[:n] {
						out[in.PID] = syscall.UTF16ToString(in.AppName[:])
					}
				}
				break
			}
		}
		pRmEndSession.Call(uintptr(session))
	}
	return out
}

type lockProc struct {
	PID       uint32   `json:"pid"`
	CT        uint64   `json:"ct"`
	Name      string   `json:"name"`
	Exe       string   `json:"exe"`
	Path      string   `json:"path"`
	Reasons   []string `json:"reasons"`
	Protected bool     `json:"locked"`
	Note      string   `json:"note,omitempty"`
	WS        uint64   `json:"ws"`
}

type analysis struct {
	Path     string     `json:"path"`
	IsDir    bool       `json:"isDir"`
	Files    int        `json:"files"`
	Dirs     int        `json:"dirs"`
	Bytes    uint64     `json:"bytes"`
	ReadOnly int        `json:"readOnly"`
	Partial  bool       `json:"partial"`
	Procs    []lockProc `json:"procs"`
	Ms       int64      `json:"ms"`
}

func normPath(p string) string {
	p = strings.TrimSpace(strings.Trim(strings.TrimSpace(p), `"`))
	if p == "" {
		return ""
	}
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	p = filepath.Clean(p)
	if len(p) > 3 {
		p = strings.TrimRight(p, `\`)
	}
	return p
}

func under(child, parent string) bool {
	c, p := strings.ToLower(child), strings.ToLower(parent)
	return c == p || strings.HasPrefix(c, strings.TrimRight(p, `\`)+`\`)
}

// checkSafePath impide borrar ubicaciones del sistema o carpetas personales raíz.
func checkSafePath(p string) error {
	lp := strings.ToLower(p)
	if len(lp) <= 3 || strings.HasSuffix(lp, `:\`) {
		return errors.New("No se puede eliminar una unidad completa.")
	}
	env := func(k string) string { return strings.ToLower(normPath(os.Getenv(k))) }
	sysroot := env("SystemRoot")
	if sysroot != "" && (under(lp, sysroot) || under(sysroot, lp)) {
		return errors.New("Esa ubicación forma parte de Windows o la contiene. Por seguridad no se puede eliminar.")
	}
	home := env("USERPROFILE")
	blocked := []string{env("ProgramFiles"), env("ProgramFiles(x86)"), env("ProgramW6432"), env("ProgramData"), home,
		env("APPDATA"), env("LOCALAPPDATA"), env("PUBLIC"), env("OneDrive")}
	if home != "" {
		for _, s := range []string{"Desktop", "Documents", "Downloads", "Pictures", "Music", "Videos", "AppData", `AppData\LocalLow`, "OneDrive", "Escritorio", "Documentos", "Descargas"} {
			blocked = append(blocked, strings.ToLower(filepath.Join(home, s)))
		}
		blocked = append(blocked, strings.ToLower(filepath.Dir(home))) // C:\Users
	}
	for _, b := range blocked {
		if b != "" && lp == b {
			return errors.New("Es una carpeta principal del sistema o de tu usuario. Elige una carpeta concreta dentro de ella.")
		}
		if b != "" && under(b, lp) {
			return errors.New("Esa carpeta contiene ubicaciones importantes del sistema o de tu usuario. Elige una carpeta más concreta.")
		}
	}
	if exe, err := os.Executable(); err == nil && under(exe, p) {
		return errors.New("Vantix Utilities se está ejecutando desde esa ubicación. Muévela a otra carpeta antes de eliminarla.")
	}
	return nil
}

func analyzePath(path string) (*analysis, error) {
	t0 := time.Now()
	path = normPath(path)
	if path == "" {
		return nil, errors.New("Indica una carpeta o archivo.")
	}
	st, err := os.Lstat(path)
	if err != nil {
		return nil, errors.New("No se encuentra la ruta indicada.")
	}
	if err := checkSafePath(path); err != nil {
		return nil, err
	}
	a := &analysis{Path: path, IsDir: st.IsDir()}
	const maxFiles = 60000
	var files []string
	if a.IsDir {
		filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if p == path {
				return nil
			}
			if d.Type()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
				a.Files++
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				a.Dirs++
				return nil
			}
			a.Files++
			if info, err := d.Info(); err == nil {
				a.Bytes += uint64(info.Size())
				if info.Mode()&0200 == 0 {
					a.ReadOnly++
				}
			}
			if len(files) < maxFiles {
				files = append(files, p)
			} else {
				a.Partial = true
			}
			return nil
		})
	} else {
		a.Files = 1
		a.Bytes = uint64(st.Size())
		files = []string{path}
	}

	reasons := map[uint32][]string{}
	add := func(pid uint32, r string) {
		for _, x := range reasons[pid] {
			if x == r {
				return
			}
		}
		reasons[pid] = append(reasons[pid], r)
	}
	for pid := range lockingProcesses(files) {
		add(pid, "Tiene archivos abiertos")
	}
	procs := snapshotProcesses()
	for _, p := range procs {
		if p.Path != "" && under(p.Path, path) {
			add(p.PID, "Se ejecuta desde esta ubicación")
		}
	}
	if a.IsDir {
		for _, p := range procs {
			if p.PID <= 4 || p.Session == 0 || p.PID == selfPID {
				continue
			}
			if cwd := processCwd(p.PID); cwd != "" && under(normPath(cwd), path) {
				add(p.PID, "Tiene la carpeta abierta como directorio de trabajo")
			}
		}
	}
	byPID := map[uint32]rawProc{}
	for _, p := range procs {
		byPID[p.PID] = p
	}
	own := descendants(procs, edgePID)
	for pid, rs := range reasons {
		p, ok := byPID[pid]
		if !ok || pid == selfPID || own[pid] {
			continue
		}
		lp := lockProc{PID: pid, CT: p.Create, Exe: p.Name, Path: p.Path, Reasons: rs, WS: p.WS}
		vi := cachedVersion(p.Path)
		lp.Name = vi.Desc
		if lp.Name == "" {
			lp.Name = strings.TrimSuffix(p.Name, filepath.Ext(p.Name))
		}
		n := strings.ToLower(p.Name)
		switch {
		case n == "explorer.exe":
			lp.Note = "Se reiniciará el Explorador de Windows automáticamente (el escritorio parpadeará un momento)."
		case criticalNames[n] || pid == 4:
			lp.Protected = true
			lp.Note = "Proceso esencial de Windows: no se finaliza. Suele liberar el archivo en unos segundos."
		case p.Session == 0 && !isAdmin():
			lp.Note = "Servicio del sistema: hace falta ejecutar la aplicación como administrador para finalizarlo."
		}
		a.Procs = append(a.Procs, lp)
	}
	sort.Slice(a.Procs, func(i, j int) bool { return a.Procs[i].Name < a.Procs[j].Name })
	a.Ms = time.Since(t0).Milliseconds()
	return a, nil
}

type deleteResult struct {
	OK        bool         `json:"ok"`
	Killed    []killResult `json:"killed"`
	Remaining int          `json:"remaining"`
	Sample    []string     `json:"sample,omitempty"`
	Error     string       `json:"error,omitempty"`
	Recycled  bool         `json:"recycled"`
	Explorer  bool         `json:"explorer"`
}

// SHFILEOPSTRUCTW
type shFileOp struct {
	Hwnd    uintptr
	Func    uint32
	From    *uint16
	To      *uint16
	Flags   uint16
	Aborted int32
	Maps    uintptr
	Title   *uint16
}

func recyclePath(p string) error {
	from, _ := syscall.UTF16FromString(p)
	from = append(from, 0)
	op := shFileOp{Func: 3, From: &from[0], Flags: 0x40 | 0x10 | 0x4 | 0x400} // FO_DELETE; ALLOWUNDO|NOCONFIRMATION|SILENT|NOERRORUI
	r, _, _ := pSHFileOperationW.Call(uintptr(unsafe.Pointer(&op)))
	if r != 0 || op.Aborted != 0 {
		return errors.New("Windows no pudo enviarlo a la Papelera")
	}
	return nil
}

func clearAttributes(path string) {
	filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		setAttrs(p, FILE_ATTRIBUTE_NORMAL)
		if d.IsDir() && p != path && d.Type()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
			return fs.SkipDir
		}
		return nil
	})
	setAttrs(path, FILE_ATTRIBUTE_NORMAL)
}

func listRemaining(path string, limit int) (int, []string) {
	n := 0
	var sample []string
	filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			n++
			if len(sample) < limit {
				sample = append(sample, p)
			}
		}
		return nil
	})
	if n == 0 {
		if _, err := os.Lstat(path); err == nil {
			n = 1
			sample = []string{path}
		}
	}
	return n, sample
}

func forceDelete(path string, targets []killTarget, recycle bool) deleteResult {
	var res deleteResult
	path = normPath(path)
	if err := checkSafePath(path); err != nil {
		res.Error = err.Error()
		return res
	}
	procs := snapshotProcesses()
	var normal []killTarget
	for _, t := range targets {
		isExplorer := false
		for _, p := range procs {
			if p.PID == t.PID && strings.EqualFold(p.Name, "explorer.exe") {
				isExplorer = true
			}
		}
		if isExplorer {
			res.Explorer = true
		} else {
			normal = append(normal, t)
		}
	}
	res.Killed = killMany(normal)
	if res.Explorer {
		for _, p := range procs {
			if strings.EqualFold(p.Name, "explorer.exe") {
				killProcess(killTarget{PID: p.PID, CT: p.Create})
			}
		}
	}
	time.Sleep(250 * time.Millisecond)

	if recycle {
		if err := recyclePath(path); err == nil {
			if _, e := os.Lstat(path); e != nil {
				res.OK, res.Recycled = true, true
			}
		}
	}
	if !res.OK {
		clearAttributes(path)
		var last error
		for i := 0; i < 6; i++ {
			last = os.RemoveAll(path)
			if _, err := os.Lstat(path); err != nil {
				last = nil
				break
			}
			time.Sleep(time.Duration(300+i*200) * time.Millisecond)
		}
		if last == nil {
			res.OK = true
		} else {
			res.Remaining, res.Sample = listRemaining(path, 8)
			res.Error = "Algunos elementos siguen bloqueados."
		}
	}
	if res.Explorer {
		go func() {
			time.Sleep(600 * time.Millisecond)
			for _, p := range snapshotProcesses() {
				if strings.EqualFold(p.Name, "explorer.exe") {
					return
				}
			}
			shellExecute("open", filepath.Join(os.Getenv("SystemRoot"), "explorer.exe"), "")
		}()
	}
	return res
}

// scheduleOnReboot marca todo lo que queda para borrarse al reiniciar (requiere administrador).
func scheduleOnReboot(path string) (int, error) {
	path = normPath(path)
	if err := checkSafePath(path); err != nil {
		return 0, err
	}
	if !isAdmin() {
		return 0, errors.New("Esta opción necesita permisos de administrador.")
	}
	var files, dirs []string
	filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && d.Type()&(fs.ModeSymlink|fs.ModeIrregular) == 0 {
			dirs = append(dirs, p)
		} else {
			files = append(files, p)
			if d.IsDir() {
				return fs.SkipDir
			}
		}
		return nil
	})
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	n := 0
	var lastErr error
	for _, f := range append(files, dirs...) {
		if err := scheduleDeleteOnReboot(f); err != nil {
			lastErr = err
		} else {
			n++
		}
	}
	if n == 0 && lastErr != nil {
		return 0, lastErr
	}
	return n, nil
}
