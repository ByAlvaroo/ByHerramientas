//go:build windows

package main

// Búsqueda de archivos y carpetas por nombre en todo el equipo.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

type hit struct {
	N string `json:"n"`
	P string `json:"p"`
	S int64  `json:"s"`
	M int64  `json:"m"`
	D bool   `json:"d"`
}

var accentRepl = strings.NewReplacer(
	"á", "a", "à", "a", "ä", "a", "â", "a", "ã", "a", "å", "a", "Á", "A", "À", "A", "Ä", "A", "Â", "A", "Ã", "A",
	"é", "e", "è", "e", "ë", "e", "ê", "e", "É", "E", "È", "E", "Ë", "E", "Ê", "E",
	"í", "i", "ì", "i", "ï", "i", "î", "i", "Í", "I", "Ì", "I", "Ï", "I", "Î", "I",
	"ó", "o", "ò", "o", "ö", "o", "ô", "o", "õ", "o", "Ó", "O", "Ò", "O", "Ö", "O", "Ô", "O", "Õ", "O",
	"ú", "u", "ù", "u", "ü", "u", "û", "u", "Ú", "U", "Ù", "U", "Ü", "U", "Û", "U",
	"ñ", "n", "Ñ", "N", "ç", "c", "Ç", "C",
)

func foldAccents(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return accentRepl.Replace(s)
		}
	}
	return s
}

type matcher struct {
	mode    string
	cs, acc bool
	q       string
	words   []string
	re      *regexp.Regexp
	exts    map[string]bool
	files   bool
	dirs    bool
}

func (m *matcher) prep(s string) string {
	if !m.cs {
		s = strings.ToLower(s)
	}
	if m.acc {
		s = foldAccents(s)
	}
	return s
}

func newMatcher(r *http.Request) (*matcher, error) {
	q := r.URL.Query()
	m := &matcher{mode: q.Get("mode"), cs: q.Get("case") == "1", acc: q.Get("acc") == "1",
		files: q.Get("files") != "0", dirs: q.Get("dirs") != "0"}
	raw := strings.TrimSpace(q.Get("q"))
	if ext := strings.TrimSpace(q.Get("ext")); ext != "" {
		m.exts = map[string]bool{}
		for _, e := range strings.FieldsFunc(ext, func(r rune) bool { return r == ',' || r == ';' || r == ' ' }) {
			m.exts["."+strings.ToLower(strings.TrimPrefix(e, "."))] = true
		}
	}
	switch m.mode {
	case "regex":
		flags := ""
		if !m.cs {
			flags = "(?i)"
		}
		re, err := regexp.Compile(flags + raw)
		if err != nil {
			return nil, fmt.Errorf("Expresión regular no válida: %v", err)
		}
		m.re = re
	case "comodin":
		pat := regexp.QuoteMeta(m.prep(raw))
		pat = strings.ReplaceAll(pat, `\*`, ".*")
		pat = strings.ReplaceAll(pat, `\?`, ".")
		m.re = regexp.MustCompile("^" + pat + "$")
	case "palabras":
		m.words = strings.Fields(m.prep(raw))
	default:
		m.q = m.prep(raw)
	}
	return m, nil
}

func (m *matcher) match(name string, dir bool) bool {
	if dir && !m.dirs || !dir && !m.files {
		return false
	}
	if m.exts != nil {
		if dir || !m.exts[strings.ToLower(filepath.Ext(name))] {
			return false
		}
	}
	switch m.mode {
	case "regex":
		return m.re.MatchString(name)
	case "comodin":
		return m.re.MatchString(m.prep(name))
	}
	n := m.prep(name)
	stem := strings.TrimSuffix(n, filepath.Ext(n))
	switch m.mode {
	case "exacto":
		return m.q == "" || n == m.q || stem == m.q
	case "empieza":
		return strings.HasPrefix(n, m.q)
	case "termina":
		return strings.HasSuffix(n, m.q) || strings.HasSuffix(stem, m.q)
	case "palabras":
		for _, w := range m.words {
			if !strings.Contains(n, w) {
				return false
			}
		}
		return true
	default:
		return strings.Contains(n, m.q)
	}
}

var sysDirs = set("windows", "$recycle.bin", "system volume information", "$winreagent", "windows.old", "$windows.~bt", "$windows.~ws", "recovery", "config.msi", "msocache", "$sysreset", "perflogs")

func handleSearch(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "sin streaming", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	send := func(ev string, v any) {
		b, _ := json.Marshal(v)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev, b)
		fl.Flush()
	}
	m, err := newMatcher(r)
	if err != nil {
		send("err", err.Error())
		return
	}
	q := r.URL.Query()
	roots := q["root"]
	hidden := q.Get("hidden") == "1"
	skipsys := q.Get("skipsys") == "1"
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 {
		limit = 10000
	}
	excl := map[string]bool{}
	for _, e := range strings.Split(q.Get("excl"), ",") {
		if e = strings.ToLower(strings.TrimSpace(e)); e != "" {
			excl[e] = true
		}
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	t0 := time.Now()
	var found, dirsN, entries, denied int64
	var curMu sync.Mutex
	cur := ""
	results := make(chan hit, 4096)

	// pila de directorios compartida por varios trabajadores
	var qmu sync.Mutex
	qcond := sync.NewCond(&qmu)
	var stack []string
	active := 0
	push := func(d string) {
		qmu.Lock()
		stack = append(stack, d)
		qmu.Unlock()
		qcond.Signal()
	}
	reason := "fin"
	var stopOnce sync.Once
	stop := func(why string) { stopOnce.Do(func() { reason = why; cancel() }) }

	workers := runtime.NumCPU() * 2
	if workers > 24 {
		workers = 24
	}
	if workers < 4 {
		workers = 4
	}
	scan := func(dir string) {
		if ctx.Err() != nil {
			return
		}
		atomic.AddInt64(&dirsN, 1)
		curMu.Lock()
		cur = dir
		curMu.Unlock()
		var fd win32FindData
		pattern := u16(longPath(strings.TrimRight(dir, `\`) + `\*`))
		// FindExInfoBasic = 1, FindExSearchNameMatch = 0, FIND_FIRST_EX_LARGE_FETCH = 2
		h, _, e := pFindFirstFileExW.Call(uintptr(unsafe.Pointer(pattern)), 1, uintptr(unsafe.Pointer(&fd)), 0, 0, 2)
		if syscall.Handle(h) == syscall.InvalidHandle {
			if e == syscall.ERROR_ACCESS_DENIED {
				atomic.AddInt64(&denied, 1)
			}
			return
		}
		defer syscall.FindClose(syscall.Handle(h))
		for {
			name := syscall.UTF16ToString(fd.FileName[:])
			if name != "." && name != ".." {
				atomic.AddInt64(&entries, 1)
				attr := fd.FileAttributes
				isDir := attr&FILE_ATTRIBUTE_DIRECTORY != 0
				skip := !hidden && attr&(FILE_ATTRIBUTE_HIDDEN|FILE_ATTRIBUTE_SYSTEM) != 0
				if !skip {
					full := strings.TrimRight(dir, `\`) + `\` + name
					if m.match(name, isDir) {
						h := hit{N: name, P: full, D: isDir, M: ftUnix(fd.LastWriteTime)}
						if !isDir {
							h.S = int64(fd.FileSizeHigh)<<32 | int64(fd.FileSizeLow)
						}
						if atomic.AddInt64(&found, 1) > int64(limit) {
							stop("limite")
							return
						}
						select {
						case results <- h:
						case <-ctx.Done():
							return
						}
					}
					ln := strings.ToLower(name)
					if isDir && attr&FILE_ATTRIBUTE_REPARSE_POINT == 0 && !excl[ln] && !(skipsys && sysDirs[ln]) {
						push(full)
					}
				}
			}
			if r, _, _ := pFindNextFileW.Call(h, uintptr(unsafe.Pointer(&fd))); r == 0 {
				break
			}
			if ctx.Err() != nil {
				return
			}
		}
	}
	for _, rt := range roots {
		rt = strings.TrimSpace(strings.Trim(rt, `"`))
		if rt != "" {
			stack = append(stack, rt)
		}
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
				d := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				active++
				qmu.Unlock()
				scan(d)
				qmu.Lock()
				active--
				qmu.Unlock()
				qcond.Broadcast()
			}
		}()
	}
	go func() { <-ctx.Done(); qcond.Broadcast() }()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	tick := time.NewTicker(120 * time.Millisecond)
	defer tick.Stop()
	batch := make([]hit, 0, 512)
	flush := func() {
		if len(batch) > 0 {
			send("r", batch)
			batch = batch[:0]
		}
	}
	progress := func() {
		curMu.Lock()
		c := cur
		curMu.Unlock()
		send("p", map[string]any{"found": min64(atomic.LoadInt64(&found), int64(limit)), "dirs": atomic.LoadInt64(&dirsN), "ms": time.Since(t0).Milliseconds(), "cur": c})
	}
loop:
	for {
		select {
		case h := <-results:
			batch = append(batch, h)
			if len(batch) >= 500 {
				flush()
			}
		case <-tick.C:
			flush()
			progress()
		case <-done:
			break loop
		case <-ctx.Done():
			break loop
		}
	}
	// recoger lo que quede en el canal
	for {
		select {
		case h := <-results:
			batch = append(batch, h)
			continue
		default:
		}
		break
	}
	if r.Context().Err() != nil {
		return // el cliente canceló
	}
	flush()
	send("done", map[string]any{"found": min64(atomic.LoadInt64(&found), int64(limit)), "dirs": atomic.LoadInt64(&dirsN),
		"entries": atomic.LoadInt64(&entries), "ms": time.Since(t0).Milliseconds(), "reason": reason, "denied": atomic.LoadInt64(&denied)})
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
