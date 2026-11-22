//go:build windows

package main

import (
	"net/http"
	"strings"
)

// registerExtra añade las rutas de las herramientas de la versión 1.1.
func registerExtra(api func(string, http.HandlerFunc)) {
	type pathReq struct {
		Path string `json:"path"`
	}

	// ---- Hardware e información del sistema ----
	api("/api/hardware", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, hardwareSnapshot()) })
	api("/api/sysinfo", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, systemReport()) })
	api("/api/memory/purge", func(w http.ResponseWriter, r *http.Request) {
		if !adminCached {
			writeJSON(w, map[string]any{"error": "Requiere permisos de administrador."})
			return
		}
		before := memStatus().AvailPhys
		if err := purgeStandby(); err != nil {
			writeJSON(w, map[string]any{"error": "No se pudo liberar: " + err.Error()})
			return
		}
		after := memStatus().AvailPhys
		var freed uint64
		if after > before {
			freed = after - before
		}
		writeJSON(w, map[string]any{"ok": true, "freed": freed})
	})

	// ---- Red ----
	api("/api/network", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, networkProcesses()) })

	// ---- Limpiador ----
	api("/api/cleaner/scan", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"cats": cleanerScan(), "admin": adminCached})
	})
	api("/api/cleaner/run", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			IDs []string `json:"ids"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		writeJSON(w, map[string]any{"results": cleanerRun(req.IDs)})
	})

	// ---- Espacio en disco ----
	api("/api/space/scan", handleSpaceScan)
	api("/api/space/node", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, spaceNode(r.URL.Query().Get("path"))) })

	// ---- Eliminar varios (duplicados, espacio) ----
	api("/api/files/delete", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Paths   []string `json:"paths"`
			Recycle bool     `json:"recycle"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		n, failed := deletePaths(req.Paths, req.Recycle)
		writeJSON(w, map[string]any{"ok": n, "failed": failed})
	})

	// ---- Duplicados ----
	api("/api/dupes", handleDupes)

	// ---- Renombrar ----
	api("/api/rename/list", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Path      string `json:"path"`
			Recursive bool   `json:"recursive"`
			Dirs      bool   `json:"dirs"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		files, err := renameList(req.Path, req.Recursive, req.Dirs)
		if err != nil {
			writeJSON(w, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"files": files})
	})
	api("/api/rename/apply", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Ops []renOp `json:"ops"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		n, errs := renameApply(req.Ops)
		writeJSON(w, map[string]any{"done": n, "errors": errs})
	})
	api("/api/rename/undo", func(w http.ResponseWriter, r *http.Request) {
		n, errs := renameUndo()
		writeJSON(w, map[string]any{"done": n, "errors": errs})
	})

	// ---- Inicio ----
	api("/api/startup", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"items": listStartup(), "admin": adminCached})
	})
	api("/api/startup/set", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     string `json:"id"`
			Enable bool   `json:"enable"`
			Delete bool   `json:"delete"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		var err error
		if req.Delete {
			err = deleteStartup(req.ID)
		} else {
			err = setStartup(req.ID, req.Enable)
		}
		if err != nil {
			msg := err.Error()
			if strings.Contains(msg, "Access is denied") {
				msg = "Acceso denegado: ejecuta la aplicación como administrador."
			}
			writeJSON(w, map[string]any{"error": msg})
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})

	// ---- Desinstalador ----
	api("/api/programs", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"programs": listPrograms(), "admin": adminCached})
	})
	api("/api/programs/uninstall", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID string `json:"id"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		if err := startUninstall(req.ID); err != nil {
			writeJSON(w, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})
	api("/api/programs/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, uninstallStatus(r.URL.Query().Get("id")))
	})
	api("/api/programs/leftovers", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"items": findLeftovers(r.URL.Query().Get("id"))})
	})
	api("/api/programs/clean", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Items []leftover `json:"items"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		n, errs := deleteLeftovers(req.Items)
		writeJSON(w, map[string]any{"done": n, "errors": errs})
	})

	// ---- Modo juego ----
	api("/api/game", func(w http.ResponseWriter, r *http.Request) {
		c := getConfig()
		writeJSON(w, map[string]any{"active": c.GameActive, "since": c.GameSince, "config": c.Game, "plans": powerPlans(), "admin": adminCached, "best": bestPlan(c.Game.PlanGUID)})
	})
	api("/api/game/on", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, gameModeOn()) })
	api("/api/game/off", func(w http.ResponseWriter, r *http.Request) {
		if err := gameModeOff(); err != nil {
			writeJSON(w, map[string]any{"error": "No se pudo restaurar el plan de energía anterior."})
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})
	api("/api/power/set", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			GUID string `json:"guid"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		if err := setPlan(req.GUID); err != nil {
			writeJSON(w, map[string]any{"error": "No se pudo cambiar el plan."})
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})
	api("/api/power/ultimate", func(w http.ResponseWriter, r *http.Request) {
		if err := addUltimatePlan(); err != nil {
			writeJSON(w, map[string]any{"error": "No se pudo añadir el plan: " + err.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})

	// ---- Actualizaciones ----
	api("/api/update/check", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, checkUpdate()) })
	api("/api/update/status", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, updateStatus()) })
	api("/api/update/install", func(w http.ResponseWriter, r *http.Request) {
		if err := installUpdate(); err != nil {
			writeJSON(w, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})
	api("/api/openurl", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			URL string `json:"url"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		ok := false
		for _, p := range []string{"https://github.com/", "https://librehardwaremonitor.org/"} {
			if strings.HasPrefix(req.URL, p) && !strings.ContainsAny(req.URL, "\"\n\r ") {
				ok = true
			}
		}
		if !ok {
			http.Error(w, "enlace no permitido", 400)
			return
		}
		shellExecute("open", req.URL, "")
		writeJSON(w, map[string]any{"ok": true})
	})
	_ = pathReq{}
}
