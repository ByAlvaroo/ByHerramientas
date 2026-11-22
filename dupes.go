//go:build windows

package main

// Buscador de archivos duplicados: agrupa por tamaño, compara un fragmento y
// confirma con el hash SHA-256 del contenido completo.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type dupFile struct {
	Path string `json:"p"`
	M    int64  `json:"m"`
}

type dupGroup struct {
	Size  int64     `json:"size"`
	Hash  string    `json:"hash"`
	Files []dupFile `json:"files"`
}

func hashFile(p string, partial bool, size int64) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if partial {
		const chunk = 64 << 10
		buf := make([]byte, chunk)
		n, _ := io.ReadFull(f, buf)
		h.Write(buf[:n])
		if size > 2*chunk {
			if _, err := f.Seek(-chunk, io.SeekEnd); err == nil {
				n, _ = io.ReadFull(f, buf)
				h.Write(buf[:n])
			}
		}
	} else {
		buf := make([]byte, 1<<20)
		if _, err := io.CopyBuffer(h, f, buf); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func handleDupes(w http.ResponseWriter, r *http.Request) {
	fl, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	send := func(ev string, v any) {
		b, _ := json.Marshal(v)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev, b)
		if fl != nil {
			fl.Flush()
		}
	}
	q := r.URL.Query()
	roots := q["root"]
	minSize, _ := strconv.ParseInt(q.Get("min"), 10, 64)
	if minSize < 1 {
		minSize = 1
	}
	hidden := q.Get("hidden") == "1"
	exts := map[string]bool{}
	for _, e := range strings.FieldsFunc(q.Get("ext"), func(r rune) bool { return r == ',' || r == ' ' || r == ';' }) {
		exts["."+strings.ToLower(strings.TrimPrefix(e, "."))] = true
	}
	ctx := r.Context()
	t0 := time.Now()
	if len(roots) == 0 {
		send("err", "Añade al menos una carpeta.")
		return
	}

	// 1) recorrer y agrupar por tamaño
	bySize := map[int64][]dupFile{}
	var scanned int64
	last := time.Now()
	for _, root := range roots {
		root = normPath(root)
		filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if ctx.Err() != nil {
				return fs.SkipAll
			}
			if err != nil {
				if d != nil && d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if d.Type()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			name := d.Name()
			if d.IsDir() {
				ln := strings.ToLower(name)
				if p != root && (sysDirs[ln] || (!hidden && strings.HasPrefix(name, "."))) {
					return fs.SkipDir
				}
				return nil
			}
			if len(exts) > 0 && !exts[strings.ToLower(filepath.Ext(name))] {
				return nil
			}
			info, err := d.Info()
			if err != nil || info.Size() < minSize {
				return nil
			}
			scanned++
			bySize[info.Size()] = append(bySize[info.Size()], dupFile{p, info.ModTime().Unix()})
			if time.Since(last) > 250*time.Millisecond {
				last = time.Now()
				send("p", map[string]any{"phase": "Buscando archivos", "n": scanned, "cur": p, "ms": time.Since(t0).Milliseconds()})
			}
			return nil
		})
	}
	if ctx.Err() != nil {
		return
	}
	// evitar el mismo archivo dos veces si las carpetas se solapan
	var candidates [][]dupFile
	var candSizes []int64
	var totalBytes int64
	for sz, l := range bySize {
		if len(l) < 2 {
			continue
		}
		seen := map[string]bool{}
		var u []dupFile
		for _, f := range l {
			k := strings.ToLower(f.Path)
			if !seen[k] {
				seen[k] = true
				u = append(u, f)
			}
		}
		if len(u) > 1 {
			candidates = append(candidates, u)
			candSizes = append(candSizes, sz)
			totalBytes += sz * int64(len(u))
		}
	}

	// 2) hash parcial y 3) hash completo, en paralelo
	type job struct {
		g    int
		i    int
		full bool
	}
	workers := runtime.NumCPU()
	if workers > 6 {
		workers = 6
	}
	var processed, done int64
	hashes := make([][]string, len(candidates))
	for i := range candidates {
		hashes[i] = make([]string, len(candidates[i]))
	}
	runPhase := func(full bool, groups [][]int, phase string) {
		jobs := make(chan job, 256)
		var wg sync.WaitGroup
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := range jobs {
					if ctx.Err() != nil {
						continue
					}
					h, err := hashFile(candidates[j.g][j.i].Path, !full, candSizes[j.g])
					if err != nil {
						h = "error:" + strconv.Itoa(j.g) + ":" + strconv.Itoa(j.i)
					}
					hashes[j.g][j.i] = h
					atomic.AddInt64(&done, 1)
					if full {
						atomic.AddInt64(&processed, candSizes[j.g])
					}
				}
			}()
		}
		stop := make(chan struct{})
		go func() {
			t := time.NewTicker(250 * time.Millisecond)
			defer t.Stop()
			for {
				select {
				case <-stop:
					return
				case <-t.C:
					send("p", map[string]any{"phase": phase, "n": atomic.LoadInt64(&done), "bytes": atomic.LoadInt64(&processed), "total": totalBytes, "ms": time.Since(t0).Milliseconds()})
				}
			}
		}()
		for g, idxs := range groups {
			for _, i := range idxs {
				jobs <- job{g, i, full}
			}
		}
		close(jobs)
		wg.Wait()
		close(stop)
	}
	all := make([][]int, len(candidates))
	for g := range candidates {
		for i := range candidates[g] {
			all[g] = append(all[g], i)
		}
	}
	runPhase(false, all, "Comparando fragmentos")
	if ctx.Err() != nil {
		return
	}
	// quedarse con los que coinciden en el fragmento
	next := make([][]int, len(candidates))
	totalBytes = 0
	for g := range candidates {
		cnt := map[string]int{}
		for _, h := range hashes[g] {
			cnt[h]++
		}
		for i, h := range hashes[g] {
			if cnt[h] > 1 && !strings.HasPrefix(h, "error:") {
				next[g] = append(next[g], i)
				totalBytes += candSizes[g]
			}
		}
	}
	atomic.StoreInt64(&done, 0)
	runPhase(true, next, "Verificando contenido")
	if ctx.Err() != nil {
		return
	}
	var groups []dupGroup
	var wasted int64
	for g, idxs := range next {
		by := map[string][]dupFile{}
		for _, i := range idxs {
			h := hashes[g][i]
			if strings.HasPrefix(h, "error:") {
				continue
			}
			by[h] = append(by[h], candidates[g][i])
		}
		for h, l := range by {
			if len(l) > 1 {
				sort.Slice(l, func(a, b int) bool { return l[a].M < l[b].M })
				groups = append(groups, dupGroup{candSizes[g], h[:16], l})
				wasted += candSizes[g] * int64(len(l)-1)
			}
		}
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Size*int64(len(groups[i].Files)-1) > groups[j].Size*int64(len(groups[j].Files)-1)
	})
	send("done", map[string]any{"groups": groups, "wasted": wasted, "scanned": scanned, "ms": time.Since(t0).Milliseconds()})
}

// deletePaths elimina (o envía a la Papelera) varios archivos.
func deletePaths(paths []string, recycle bool) (int, []string) {
	ok := 0
	var failed []string
	for _, p := range paths {
		p = normPath(p)
		if checkSafePath(p) != nil {
			failed = append(failed, p)
			continue
		}
		var err error
		var size int64
		isDir := false
		if st, e := os.Lstat(p); e == nil {
			size, isDir = st.Size(), st.IsDir()
		}
		if recycle {
			err = recyclePath(p)
		} else {
			setAttrs(p, FILE_ATTRIBUTE_NORMAL)
			err = os.RemoveAll(p)
		}
		if _, e := os.Lstat(p); err == nil && e != nil {
			ok++
			spaceForget(p, size, isDir)
		} else {
			failed = append(failed, p)
		}
	}
	return ok, failed
}
