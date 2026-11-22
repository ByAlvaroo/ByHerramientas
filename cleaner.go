//go:build windows

package main

// Limpiador: temporales, cachés de navegadores y aplicaciones, informes de errores,
// caché de Windows Update, miniaturas y Papelera.

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"
)

var (
	pSHQueryRecycleBinW = shell32.NewProc("SHQueryRecycleBinW")
	pSHEmptyRecycleBinW = shell32.NewProc("SHEmptyRecycleBinW")
)

type cleanCat struct {
	ID      string   `json:"id"`
	Group   string   `json:"group"`
	Name    string   `json:"name"`
	Desc    string   `json:"desc"`
	Admin   bool     `json:"admin"`
	Default bool     `json:"default"`
	Warn    string   `json:"warn,omitempty"`
	Size    uint64   `json:"size"`
	Files   int      `json:"files"`
	Paths   []string `json:"paths"`
	Running string   `json:"running,omitempty"` // proceso abierto que impide limpiar todo
	glob    []string // patrones de archivo (para categorías de archivos sueltos)
	minAge  time.Duration
}

func env(k string) string { return os.Getenv(k) }

// chromiumCaches devuelve las carpetas de caché de todos los perfiles de un navegador Chromium.
func chromiumCaches(userData string) []string {
	var out []string
	entries, err := os.ReadDir(userData)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		n := e.Name()
		if !e.IsDir() || !(n == "Default" || strings.HasPrefix(n, "Profile ") || n == "Guest Profile") {
			continue
		}
		for _, sub := range []string{`Cache\Cache_Data`, "Code Cache", "GPUCache", `Service Worker\CacheStorage`, "DawnGraphiteCache", "DawnWebGPUCache"} {
			out = append(out, filepath.Join(userData, n, sub))
		}
	}
	for _, sub := range []string{"ShaderCache", "GrShaderCache", "GraphiteDawnCache"} {
		out = append(out, filepath.Join(userData, sub))
	}
	return out
}

func firefoxCaches() []string {
	var out []string
	base := filepath.Join(env("LOCALAPPDATA"), `Mozilla\Firefox\Profiles`)
	entries, _ := os.ReadDir(base)
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, filepath.Join(base, e.Name(), "cache2"), filepath.Join(base, e.Name(), "startupCache"), filepath.Join(base, e.Name(), "thumbnails"))
		}
	}
	return out
}

func cleanCategories() []*cleanCat {
	la, ra, win, pd := env("LOCALAPPDATA"), env("APPDATA"), env("SystemRoot"), env("ProgramData")
	cats := []*cleanCat{
		{ID: "temp_user", Group: "Windows", Name: "Archivos temporales del usuario", Desc: "Archivos que los programas dejan en tu carpeta TEMP. Se omiten los creados en la última hora.", Default: true, Paths: []string{env("TEMP")}, minAge: time.Hour},
		{ID: "temp_win", Group: "Windows", Name: "Archivos temporales de Windows", Desc: "Carpeta Temp del sistema.", Admin: true, Default: true, Paths: []string{filepath.Join(win, "Temp")}, minAge: time.Hour},
		{ID: "wu", Group: "Windows", Name: "Caché de Windows Update", Desc: "Actualizaciones ya descargadas e instaladas. Windows las vuelve a descargar si las necesita.", Admin: true, Default: true, Paths: []string{filepath.Join(win, `SoftwareDistribution\Download`)}},
		{ID: "delivery", Group: "Windows", Name: "Optimización de distribución", Desc: "Copias de actualizaciones compartidas con otros equipos.", Admin: true, Default: true, Paths: []string{filepath.Join(win, `ServiceProfiles\NetworkService\AppData\Local\Microsoft\Windows\DeliveryOptimization\Cache`)}},
		{ID: "errors", Group: "Windows", Name: "Informes de errores y volcados", Desc: "Informes de fallos de programas y volcados de memoria.", Default: true, Paths: []string{
			filepath.Join(la, "CrashDumps"), filepath.Join(la, `Microsoft\Windows\WER\ReportArchive`), filepath.Join(la, `Microsoft\Windows\WER\ReportQueue`),
			filepath.Join(pd, `Microsoft\Windows\WER\ReportArchive`), filepath.Join(pd, `Microsoft\Windows\WER\ReportQueue`), filepath.Join(win, "Minidump"), filepath.Join(win, "LiveKernelReports")}},
		{ID: "thumbs", Group: "Windows", Name: "Caché de miniaturas", Desc: "Vistas previas de imágenes y vídeos del Explorador. Se regeneran al abrir las carpetas.", Default: false, Paths: []string{filepath.Join(la, `Microsoft\Windows\Explorer`)}, glob: []string{"thumbcache_*.db", "iconcache_*.db"}},
		{ID: "logs", Group: "Windows", Name: "Registros antiguos de Windows", Desc: "Archivos de registro (.log, .etl) de más de 7 días.", Admin: true, Default: false, Paths: []string{filepath.Join(win, "Logs"), filepath.Join(win, `Panther`)}, glob: []string{"*.log", "*.etl", "*.cab"}, minAge: 7 * 24 * time.Hour},
		{ID: "shader", Group: "Windows", Name: "Caché de sombreadores (DirectX, NVIDIA, AMD)", Desc: "Se regenera automáticamente. Los juegos pueden tardar algo más en cargar la primera vez.", Default: false, Paths: []string{
			filepath.Join(la, "D3DSCache"), filepath.Join(la, `NVIDIA\DXCache`), filepath.Join(la, `NVIDIA\GLCache`), filepath.Join(la, `AMD\DxCache`), filepath.Join(la, `AMD\DxcCache`), filepath.Join(la, `AMD\VkCache`), filepath.Join(la, `AMD\GLCache`)}},
		{ID: "chrome", Group: "Navegadores", Name: "Google Chrome", Desc: "Caché de páginas, imágenes y scripts. No borra historial, contraseñas ni sesiones.", Default: true, Paths: chromiumCaches(filepath.Join(la, `Google\Chrome\User Data`)), Running: "chrome.exe"},
		{ID: "edge", Group: "Navegadores", Name: "Microsoft Edge", Desc: "Caché de páginas, imágenes y scripts. No borra historial, contraseñas ni sesiones.", Default: true, Paths: chromiumCaches(filepath.Join(la, `Microsoft\Edge\User Data`)), Running: "msedge.exe"},
		{ID: "brave", Group: "Navegadores", Name: "Brave", Desc: "Caché del navegador.", Default: true, Paths: chromiumCaches(filepath.Join(la, `BraveSoftware\Brave-Browser\User Data`)), Running: "brave.exe"},
		{ID: "opera", Group: "Navegadores", Name: "Opera / Opera GX", Desc: "Caché del navegador.", Default: true, Paths: append(append(chromiumCaches(filepath.Join(ra, `Opera Software\Opera Stable`)), chromiumCaches(filepath.Join(ra, `Opera Software\Opera GX Stable`))...), filepath.Join(la, `Opera Software\Opera Stable\Cache`), filepath.Join(la, `Opera Software\Opera GX Stable\Cache`)), Running: "opera.exe"},
		{ID: "vivaldi", Group: "Navegadores", Name: "Vivaldi", Desc: "Caché del navegador.", Default: true, Paths: chromiumCaches(filepath.Join(la, `Vivaldi\User Data`)), Running: "vivaldi.exe"},
		{ID: "firefox", Group: "Navegadores", Name: "Mozilla Firefox", Desc: "Caché del navegador.", Default: true, Paths: firefoxCaches(), Running: "firefox.exe"},
		{ID: "discord", Group: "Aplicaciones", Name: "Discord", Desc: "Caché de imágenes y vídeos.", Default: true, Paths: []string{filepath.Join(ra, `discord\Cache\Cache_Data`), filepath.Join(ra, `discord\Code Cache`), filepath.Join(ra, `discord\GPUCache`)}, Running: "discord.exe"},
		{ID: "spotify", Group: "Aplicaciones", Name: "Spotify", Desc: "Canciones guardadas en caché para escucharlas más rápido (no borra las descargas sin conexión).", Default: false, Paths: []string{filepath.Join(la, `Spotify\Data`), filepath.Join(la, `Spotify\Browser\Cache`)}, Running: "spotify.exe"},
		{ID: "steam", Group: "Aplicaciones", Name: "Steam (caché web)", Desc: "Caché de la tienda y la biblioteca.", Default: true, Paths: []string{filepath.Join(la, `Steam\htmlcache`)}, Running: "steamwebhelper.exe"},
		{ID: "teams", Group: "Aplicaciones", Name: "Microsoft Teams", Desc: "Caché de Teams.", Default: true, Paths: []string{filepath.Join(la, `Packages\MSTeams_8wekyb3d8bbwe\LocalCache\Microsoft\MSTeams\EBWebView\Default\Cache`), filepath.Join(ra, `Microsoft\Teams\Cache`)}, Running: "ms-teams.exe"},
		{ID: "recycle", Group: "Papelera", Name: "Papelera de reciclaje", Desc: "Vacía la Papelera de todas las unidades. No se podrá recuperar lo que contiene.", Default: false},
	}
	return cats
}

func (c *cleanCat) matches(name string) bool {
	if len(c.glob) == 0 {
		return true
	}
	ln := strings.ToLower(name)
	for _, g := range c.glob {
		if ok, _ := filepath.Match(strings.ToLower(g), ln); ok {
			return true
		}
	}
	return false
}

// walkCat recorre los archivos de una categoría; fn devuelve false para detenerse.
func (c *cleanCat) walk(fn func(path string, info fs.FileInfo)) {
	now := time.Now()
	for _, root := range c.Paths {
		if root == "" {
			continue
		}
		if _, err := os.Stat(root); err != nil {
			continue
		}
		recursive := len(c.glob) == 0 || c.ID == "logs"
		filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
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
			if d.IsDir() {
				if p != root && !recursive {
					return fs.SkipDir
				}
				return nil
			}
			if !c.matches(d.Name()) {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			if c.minAge > 0 && now.Sub(info.ModTime()) < c.minAge {
				return nil
			}
			fn(p, info)
			return nil
		})
	}
}

func recycleBinInfo() (uint64, int) {
	var q struct {
		Cb    uint32
		_     uint32
		Size  int64
		Items int64
	}
	q.Cb = uint32(unsafe.Sizeof(q))
	if r, _, _ := pSHQueryRecycleBinW.Call(0, uintptr(unsafe.Pointer(&q))); r != 0 {
		return 0, 0
	}
	return uint64(q.Size), int(q.Items)
}

func runningNames() map[string]bool {
	m := map[string]bool{}
	for _, p := range snapshotProcesses() {
		m[strings.ToLower(p.Name)] = true
	}
	return m
}

func cleanerScan() []*cleanCat {
	cats := cleanCategories()
	running := runningNames()
	var wg sync.WaitGroup
	for _, c := range cats {
		if c.Running != "" && !running[c.Running] {
			c.Running = ""
		}
		if c.ID == "recycle" {
			c.Size, c.Files = recycleBinInfo()
			continue
		}
		wg.Add(1)
		go func(c *cleanCat) {
			defer wg.Done()
			c.walk(func(_ string, info fs.FileInfo) {
				c.Size += uint64(info.Size())
				c.Files++
			})
		}(c)
	}
	wg.Wait()
	// no mostrar rutas internas largas en la interfaz: solo las que existen
	for _, c := range cats {
		var ex []string
		for _, p := range c.Paths {
			if _, err := os.Stat(p); err == nil {
				ex = append(ex, p)
			}
		}
		c.Paths = ex
	}
	return cats
}

type cleanResult struct {
	ID      string `json:"id"`
	Freed   uint64 `json:"freed"`
	Deleted int    `json:"deleted"`
	Skipped int    `json:"skipped"`
	Error   string `json:"error,omitempty"`
}

func cleanerRun(ids []string) []cleanResult {
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var out []cleanResult
	for _, c := range cleanCategories() {
		if !want[c.ID] {
			continue
		}
		res := cleanResult{ID: c.ID}
		if c.Admin && !adminCached {
			res.Error = "Requiere administrador"
			out = append(out, res)
			continue
		}
		if c.ID == "recycle" {
			size, n := recycleBinInfo()
			if r, _, _ := pSHEmptyRecycleBinW.Call(0, 0, 7); r == 0 || n == 0 {
				res.Freed, res.Deleted = size, n
			} else {
				res.Error = "No se pudo vaciar la Papelera"
			}
			out = append(out, res)
			continue
		}
		var dirs []string
		c.walk(func(p string, info fs.FileInfo) {
			if info.Mode()&0200 == 0 {
				setAttrs(p, FILE_ATTRIBUTE_NORMAL)
			}
			if os.Remove(p) == nil {
				res.Freed += uint64(info.Size())
				res.Deleted++
			} else {
				res.Skipped++
			}
		})
		// eliminar subcarpetas vacías (nunca la carpeta raíz de la categoría)
		if len(c.glob) == 0 {
			for _, root := range c.Paths {
				filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
					if err == nil && d.IsDir() && p != root && d.Type()&fs.ModeSymlink == 0 {
						dirs = append(dirs, p)
					}
					return nil
				})
			}
			for i := len(dirs) - 1; i >= 0; i-- {
				os.Remove(dirs[i]) // solo se borra si está vacía
			}
		}
		out = append(out, res)
	}
	return out
}
