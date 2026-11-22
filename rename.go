//go:build windows

package main

// Renombrado en masa con vista previa y opción de deshacer.

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type renFile struct {
	Name  string `json:"name"`
	Dir   string `json:"dir"`
	Size  int64  `json:"size"`
	M     int64  `json:"m"`
	IsDir bool   `json:"isDir"`
}

func renameList(folder string, recursive, includeDirs bool) ([]renFile, error) {
	folder = normPath(folder)
	st, err := os.Stat(folder)
	if err != nil || !st.IsDir() {
		return nil, errors.New("La carpeta no existe.")
	}
	var out []renFile
	filepath.WalkDir(folder, func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == folder {
			return nil
		}
		if d.IsDir() {
			if includeDirs && (recursive || filepath.Dir(p) == folder) {
				if info, e := d.Info(); e == nil {
					out = append(out, renFile{d.Name(), filepath.Dir(p), 0, info.ModTime().Unix(), true})
				}
			}
			if !recursive {
				return fs.SkipDir
			}
			return nil
		}
		if info, e := d.Info(); e == nil {
			out = append(out, renFile{d.Name(), filepath.Dir(p), info.Size(), info.ModTime().Unix(), false})
		}
		if len(out) >= 20000 {
			return fs.SkipAll
		}
		return nil
	})
	sort.Slice(out, func(i, j int) bool {
		if out[i].Dir != out[j].Dir {
			return out[i].Dir < out[j].Dir
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

type renOp struct {
	Dir  string `json:"dir"`
	From string `json:"from"`
	To   string `json:"to"`
}

var badChars = `<>:"/\|?*`

func validName(n string) bool {
	if n == "" || n == "." || n == ".." || strings.ContainsAny(n, badChars) || strings.HasSuffix(n, " ") || strings.HasSuffix(n, ".") {
		return false
	}
	for _, r := range n {
		if r < 32 {
			return false
		}
	}
	base := strings.ToUpper(strings.TrimSuffix(n, filepath.Ext(n)))
	switch base {
	case "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "LPT1", "LPT2", "LPT3":
		return false
	}
	return true
}

func undoFile() string { return filepath.Join(dataDir(), "rename-undo.json") }

// renameApply renombra en dos fases (nombre temporal y nombre final) para evitar choques
// como intercambiar «a» y «b». Guarda un registro para deshacer.
func renameApply(ops []renOp) (int, []string) {
	var errs []string
	var valid []renOp
	targets := map[string]bool{}
	for _, o := range ops {
		if o.From == o.To {
			continue
		}
		if !validName(o.To) {
			errs = append(errs, o.To+": nombre no válido")
			continue
		}
		k := strings.ToLower(filepath.Join(o.Dir, o.To))
		if targets[k] {
			errs = append(errs, o.To+": nombre repetido")
			continue
		}
		targets[k] = true
		valid = append(valid, o)
	}
	sources := map[string]bool{}
	for _, o := range valid {
		sources[strings.ToLower(filepath.Join(o.Dir, o.From))] = true
	}
	// un destino que ya existe y no se va a renombrar es un conflicto
	var final []renOp
	for _, o := range valid {
		to := filepath.Join(o.Dir, o.To)
		if _, err := os.Lstat(to); err == nil && !sources[strings.ToLower(to)] && !strings.EqualFold(o.From, o.To) {
			errs = append(errs, o.To+": ya existe un archivo con ese nombre")
			continue
		}
		final = append(final, o)
	}
	// carpetas más profundas primero para no invalidar rutas
	sort.SliceStable(final, func(i, j int) bool { return len(final[i].Dir) > len(final[j].Dir) })
	stamp := time.Now().UnixNano()
	type tmp struct {
		op  renOp
		tmp string
	}
	var phase []tmp
	for i, o := range final {
		t := filepath.Join(o.Dir, ".vxren_"+itoa(stamp)+"_"+itoa(int64(i)))
		if err := os.Rename(filepath.Join(o.Dir, o.From), t); err != nil {
			errs = append(errs, o.From+": "+friendlyErr(err))
			continue
		}
		phase = append(phase, tmp{o, t})
	}
	var done []renOp
	for _, p := range phase {
		if err := os.Rename(p.tmp, filepath.Join(p.op.Dir, p.op.To)); err != nil {
			os.Rename(p.tmp, filepath.Join(p.op.Dir, p.op.From))
			errs = append(errs, p.op.From+": "+friendlyErr(err))
			continue
		}
		done = append(done, p.op)
	}
	if len(done) > 0 {
		b, _ := json.Marshal(done)
		os.WriteFile(undoFile(), b, 0644)
	}
	return len(done), errs
}

func renameUndo() (int, []string) {
	b, err := os.ReadFile(undoFile())
	if err != nil {
		return 0, []string{"No hay ningún renombrado que deshacer."}
	}
	var ops []renOp
	json.Unmarshal(b, &ops)
	var rev []renOp
	for _, o := range ops {
		rev = append(rev, renOp{o.Dir, o.To, o.From})
	}
	n, errs := renameApply(rev)
	os.Remove(undoFile())
	return n, errs
}

func friendlyErr(err error) string {
	s := err.Error()
	switch {
	case strings.Contains(s, "used by another process"), strings.Contains(s, "being used"):
		return "está en uso por otro programa"
	case strings.Contains(s, "Access is denied"), strings.Contains(s, "denied"):
		return "acceso denegado"
	}
	return s
}
