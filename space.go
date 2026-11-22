//go:build windows

package main

// Analizador de espacio: recorre una unidad o carpeta y calcula cuánto ocupa cada carpeta.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

type dnode struct {
	name     string
	parent   *dnode
	own      int64 // archivos directamente en esta carpeta
	ownN     int32
	size     int64 // total (calculado al final)
	files    int64
	children []*dnode
	denied   bool
}

type bigFile struct {
	Path string `json:"p"`
	Size int64  `json:"s"`
	M    int64  `json:"m"`
}

type spaceScan struct {
	root    *dnode
	rootP   string
	done    bool
	started time.Time
	ms      int64
	types   map[string]int64
	big     []bigFile
	dirs    int64
	files   int64
	cancel  func()
}

var (
	spMu  sync.Mutex
	spCur *spaceScan
)

var typeOf = func() map[string]string {
	m := map[string]string{}
	add := func(cat string, exts ...string) {
		for _, e := range exts {
			m[e] = cat
		}
	}
	add("Vídeo", ".mp4", ".mkv", ".avi", ".mov", ".wmv", ".webm", ".flv", ".m4v", ".mpg", ".mpeg", ".ts", ".m2ts")
	add("Imágenes", ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".heic", ".tif", ".tiff", ".raw", ".cr2", ".nef", ".arw", ".psd", ".svg", ".ico")
	add("Audio", ".mp3", ".wav", ".flac", ".ogg", ".m4a", ".aac", ".wma", ".opus")
	add("Documentos", ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".odt", ".ods", ".txt", ".md", ".epub", ".csv", ".rtf")
	add("Comprimidos", ".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz", ".iso", ".img", ".cab")
	add("Programas", ".exe", ".msi", ".dll", ".sys", ".jar", ".apk", ".msix", ".appx")
	add("Juegos y recursos", ".pak", ".vpk", ".bnk", ".wem", ".uasset", ".ucas", ".utoc", ".bsa", ".ba2", ".dat", ".bin", ".gcf", ".forge", ".rpf")
	add("Código y datos", ".js", ".ts", ".py", ".go", ".java", ".c", ".cpp", ".h", ".cs", ".json", ".xml", ".db", ".sqlite", ".log")
	add("Discos virtuales", ".vhd", ".vhdx", ".vmdk", ".vdi", ".qcow2")
	return m
}()

func (n *dnode) path() string {
	var parts []string
	for x := n; x != nil; x = x.parent {
		parts = append(parts, x.name)
	}
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	p := parts[0]
	for _, s := range parts[1:] {
		p = strings.TrimRight(p, `\`) + `\` + s
	}
	return p
}

func handleSpaceScan(w http.ResponseWriter, r *http.Request) {
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
	rootP := normPath(r.URL.Query().Get("root"))
	if rootP == "" {
		send("err", "Indica una unidad o carpeta.")
		return
	}
	ctx := r.Context()
	sc := &spaceScan{root: &dnode{name: rootP}, rootP: rootP, started: time.Now(), types: map[string]int64{}}
	spMu.Lock()
	spCur = sc // la anterior se libera
	spMu.Unlock()

	var qmu sync.Mutex
	qcond := sync.NewCond(&qmu)
	stack := []*dnode{sc.root}
	active := 0
	var typesMu sync.Mutex
	var bigMu sync.Mutex
	var minBig int64
	cur := atomic.Value{}
	cur.Store("")

	scan := func(n *dnode) {
		dir := n.path()
		cur.Store(dir)
		atomic.AddInt64(&sc.dirs, 1)
		var fd win32FindData
		h, _, _ := pFindFirstFileExW.Call(uintptr(unsafe.Pointer(u16(longPath(strings.TrimRight(dir, `\`)+`\*`)))), 1, uintptr(unsafe.Pointer(&fd)), 0, 0, 2)
		if syscall.Handle(h) == syscall.InvalidHandle {
			n.denied = true
			return
		}
		defer syscall.FindClose(syscall.Handle(h))
		localTypes := map[string]int64{}
		var kids []*dnode
		for {
			name := syscall.UTF16ToString(fd.FileName[:])
			if name != "." && name != ".." {
				if fd.FileAttributes&FILE_ATTRIBUTE_DIRECTORY != 0 {
					if fd.FileAttributes&FILE_ATTRIBUTE_REPARSE_POINT == 0 {
						kids = append(kids, &dnode{name: name, parent: n})
					}
				} else {
					sz := int64(fd.FileSizeHigh)<<32 | int64(fd.FileSizeLow)
					n.own += sz
					n.ownN++
					atomic.AddInt64(&sc.files, 1)
					var ext string
					if i := strings.LastIndexByte(name, '.'); i >= 0 {
						ext = strings.ToLower(name[i:])
					}
					t := typeOf[ext]
					if t == "" {
						t = "Otros"
					}
					localTypes[t] += sz
					if sz > atomic.LoadInt64(&minBig) {
						bigMu.Lock()
						sc.big = append(sc.big, bigFile{strings.TrimRight(dir, `\`) + `\` + name, sz, ftUnix(fd.LastWriteTime)})
						if len(sc.big) > 400 {
							sort.Slice(sc.big, func(i, j int) bool { return sc.big[i].Size > sc.big[j].Size })
							sc.big = sc.big[:200]
							atomic.StoreInt64(&minBig, sc.big[len(sc.big)-1].Size)
						}
						bigMu.Unlock()
					}
				}
			}
			if r, _, _ := pFindNextFileW.Call(h, uintptr(unsafe.Pointer(&fd))); r == 0 || ctx.Err() != nil {
				break
			}
		}
		n.children = kids
		typesMu.Lock()
		for k, v := range localTypes {
			sc.types[k] += v
		}
		typesMu.Unlock()
		if len(kids) > 0 {
			qmu.Lock()
			stack = append(stack, kids...)
			qmu.Unlock()
			qcond.Broadcast()
		}
	}
	workers := runtime.NumCPU() * 2
	if workers > 16 {
		workers = 16
	}
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				qmu.Lock()
				for len(stack) == 0 && active > 0 && ctx.Err() == nil {
					qcond.Wait()
				}
				if len(stack) == 0 || ctx.Err() != nil {
					qmu.Unlock()
					qcond.Broadcast()
					return
				}
				n := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				active++
				qmu.Unlock()
				scan(n)
				qmu.Lock()
				active--
				qmu.Unlock()
				qcond.Broadcast()
			}
		}()
	}
	go func() { <-ctx.Done(); qcond.Broadcast() }()
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	t := time.NewTicker(250 * time.Millisecond)
	defer t.Stop()
loop:
	for {
		select {
		case <-done:
			break loop
		case <-ctx.Done():
			return
		case <-t.C:
			send("p", map[string]any{"dirs": atomic.LoadInt64(&sc.dirs), "files": atomic.LoadInt64(&sc.files), "ms": time.Since(sc.started).Milliseconds(), "cur": cur.Load()})
		}
	}
	// totales (post-orden iterativo)
	var order []*dnode
	st := []*dnode{sc.root}
	for len(st) > 0 {
		n := st[len(st)-1]
		st = st[:len(st)-1]
		order = append(order, n)
		st = append(st, n.children...)
	}
	for i := len(order) - 1; i >= 0; i-- {
		n := order[i]
		n.size += n.own
		n.files += int64(n.ownN)
		sort.Slice(n.children, func(a, b int) bool { return n.children[a].size > n.children[b].size })
		if n.parent != nil {
			n.parent.size += n.size
			n.parent.files += n.files
		}
	}
	sort.Slice(sc.big, func(i, j int) bool { return sc.big[i].Size > sc.big[j].Size })
	if len(sc.big) > 200 {
		sc.big = sc.big[:200]
	}
	sc.done = true
	sc.ms = time.Since(sc.started).Milliseconds()
	free, total, _ := diskSpace(volRoot(rootP))
	send("done", map[string]any{"dirs": sc.dirs, "files": sc.files, "ms": sc.ms, "size": sc.root.size, "free": free, "total": total})
}

func (sc *spaceScan) find(p string) *dnode {
	p = normPath(p)
	if strings.EqualFold(p, sc.rootP) {
		return sc.root
	}
	if !under(p, sc.rootP) {
		return nil
	}
	rest := strings.TrimPrefix(strings.ToLower(p), strings.ToLower(strings.TrimRight(sc.rootP, `\`))+`\`)
	n := sc.root
	for _, part := range strings.Split(rest, `\`) {
		var next *dnode
		for _, c := range n.children {
			if strings.EqualFold(c.name, part) {
				next = c
				break
			}
		}
		if next == nil {
			return nil
		}
		n = next
	}
	return n
}

type spaceItem struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Files  int64  `json:"files"`
	Dir    bool   `json:"dir"`
	Denied bool   `json:"denied,omitempty"`
	Kids   int    `json:"kids"`
}

// spaceNode devuelve el contenido de una carpeta ya analizada: subcarpetas y sus archivos.
func spaceNode(p string) map[string]any {
	spMu.Lock()
	sc := spCur
	spMu.Unlock()
	if sc == nil || !sc.done {
		return map[string]any{"error": "Primero analiza una unidad o carpeta."}
	}
	n := sc.find(p)
	if n == nil {
		return map[string]any{"error": "Esa carpeta no forma parte del análisis."}
	}
	var items []spaceItem
	for _, c := range n.children {
		items = append(items, spaceItem{Name: c.name, Path: c.path(), Size: c.size, Files: c.files, Dir: true, Denied: c.denied, Kids: len(c.children)})
	}
	// archivos de esta carpeta (lectura directa, solo los 300 más grandes)
	dir := n.path()
	var files []spaceItem
	var fd win32FindData
	h, _, _ := pFindFirstFileExW.Call(uintptr(unsafe.Pointer(u16(longPath(strings.TrimRight(dir, `\`)+`\*`)))), 1, uintptr(unsafe.Pointer(&fd)), 0, 0, 2)
	if syscall.Handle(h) != syscall.InvalidHandle {
		for {
			if fd.FileAttributes&FILE_ATTRIBUTE_DIRECTORY == 0 {
				name := syscall.UTF16ToString(fd.FileName[:])
				files = append(files, spaceItem{Name: name, Path: strings.TrimRight(dir, `\`) + `\` + name, Size: int64(fd.FileSizeHigh)<<32 | int64(fd.FileSizeLow)})
			}
			if r, _, _ := pFindNextFileW.Call(h, uintptr(unsafe.Pointer(&fd))); r == 0 {
				break
			}
		}
		syscall.FindClose(syscall.Handle(h))
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Size > files[j].Size })
	if len(files) > 300 {
		var rest int64
		for _, f := range files[300:] {
			rest += f.Size
		}
		files = append(files[:300], spaceItem{Name: fmt.Sprintf("%d archivos más pequeños", len(files)-300), Size: rest})
	}
	items = append(items, files...)
	sort.SliceStable(items, func(i, j int) bool { return items[i].Size > items[j].Size })
	var crumbs []map[string]string
	for x := n; x != nil; x = x.parent {
		crumbs = append([]map[string]string{{"name": x.name, "path": x.path()}}, crumbs...)
	}
	resp := map[string]any{"path": dir, "size": n.size, "files": n.files, "items": items, "crumbs": crumbs}
	if n == sc.root {
		type tsz struct {
			Name string `json:"name"`
			Size int64  `json:"size"`
		}
		var types []tsz
		for k, v := range sc.types {
			types = append(types, tsz{k, v})
		}
		sort.Slice(types, func(i, j int) bool { return types[i].Size > types[j].Size })
		resp["types"] = types
		resp["big"] = sc.big
		free, total, _ := diskSpace(volRoot(sc.rootP))
		resp["free"], resp["total"] = free, total
	}
	return resp
}

// spaceForget quita del árbol un elemento borrado (para no volver a analizar).
func spaceForget(p string, size int64, isDir bool) {
	spMu.Lock()
	sc := spCur
	spMu.Unlock()
	if sc == nil || !sc.done {
		return
	}
	parentP := p[:strings.LastIndexByte(p, '\\')]
	if len(parentP) == 2 {
		parentP += `\`
	}
	par := sc.find(parentP)
	if par == nil {
		return
	}
	if isDir {
		for i, c := range par.children {
			if strings.EqualFold(c.path(), p) {
				size = c.size
				par.children = append(par.children[:i], par.children[i+1:]...)
				break
			}
		}
	} else {
		par.own -= size
	}
	for x := par; x != nil; x = x.parent {
		x.size -= size
	}
}

func volRoot(p string) string {
	if len(p) >= 2 && p[1] == ':' {
		return p[:2] + `\`
	}
	return p
}
