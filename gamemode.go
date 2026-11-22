//go:build windows

package main

// Modo juego: cierra lo innecesario, cambia a un plan de energía de alto rendimiento
// y libera la memoria en espera. Al desactivarlo se restaura el plan anterior.

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var (
	powrprof               = syscall.NewLazyDLL("powrprof.dll")
	pPowerGetActiveScheme  = powrprof.NewProc("PowerGetActiveScheme")
	pPowerSetActiveScheme  = powrprof.NewProc("PowerSetActiveScheme")
	pPowerEnumerate        = powrprof.NewProc("PowerEnumerate")
	pPowerReadFriendlyName = powrprof.NewProc("PowerReadFriendlyName")
	pLocalFree             = kernel32.NewProc("LocalFree")
)

const (
	guidBalanced = "381b4222-f694-41f0-9685-ff5bb260df2e"
	guidHigh     = "8c5e7fda-e8bf-4a96-9a85-a6e23a8c635c"
	guidUltimate = "e9a42b02-d5df-448d-aa00-03f14749eb61"
	guidSaver    = "a1841308-3541-4fab-bc81-f71556f20b4a"
)

func guidString(g guid) string {
	return fmt.Sprintf("%08x-%04x-%04x-%02x%02x-%02x%02x%02x%02x%02x%02x", g.D1, g.D2, g.D3, g.D4[0], g.D4[1], g.D4[2], g.D4[3], g.D4[4], g.D4[5], g.D4[6], g.D4[7])
}

func parseGUID(s string) (guid, bool) {
	var g guid
	var b [8]uint32
	n, err := fmt.Sscanf(s, "%08x-%04x-%04x-%02x%02x-%02x%02x%02x%02x%02x%02x", &g.D1, &g.D2, &g.D3, &b[0], &b[1], &b[2], &b[3], &b[4], &b[5], &b[6], &b[7])
	if err != nil || n != 11 {
		return g, false
	}
	for i := range b {
		g.D4[i] = byte(b[i])
	}
	return g, true
}

type powerPlan struct {
	GUID   string `json:"guid"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
	Kind   string `json:"kind"`
}

func activePlan() string {
	var p *guid
	if r, _, _ := pPowerGetActiveScheme.Call(0, uintptr(unsafe.Pointer(&p))); r != 0 || p == nil {
		return ""
	}
	s := guidString(*p)
	pLocalFree.Call(uintptr(unsafe.Pointer(p)))
	return s
}

func powerPlans() []powerPlan {
	if powrprof.Load() != nil {
		return nil
	}
	act := activePlan()
	var out []powerPlan
	for i := 0; i < 32; i++ {
		var g guid
		size := uint32(unsafe.Sizeof(g))
		if r, _, _ := pPowerEnumerate.Call(0, 0, 0, 16, uintptr(i), uintptr(unsafe.Pointer(&g)), uintptr(unsafe.Pointer(&size))); r != 0 {
			break
		}
		var n uint32
		pPowerReadFriendlyName.Call(0, uintptr(unsafe.Pointer(&g)), 0, 0, 0, uintptr(unsafe.Pointer(&n)))
		name := ""
		if n > 0 && n < 4096 {
			buf := make([]uint16, n/2+1)
			pPowerReadFriendlyName.Call(0, uintptr(unsafe.Pointer(&g)), 0, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)))
			name = syscall.UTF16ToString(buf)
		}
		gs := guidString(g)
		kind := ""
		switch gs {
		case guidBalanced:
			kind = "balanced"
		case guidHigh:
			kind = "high"
		case guidUltimate:
			kind = "ultimate"
		case guidSaver:
			kind = "saver"
		}
		out = append(out, powerPlan{gs, name, gs == act, kind})
	}
	return out
}

func setPlan(gs string) error {
	g, ok := parseGUID(gs)
	if !ok {
		return errors.New("plan no válido")
	}
	if r, _, _ := pPowerSetActiveScheme.Call(0, uintptr(unsafe.Pointer(&g))); r != 0 {
		return syscall.Errno(r)
	}
	return nil
}

// addUltimatePlan crea el plan «Máximo rendimiento» (oculto por defecto en Windows).
func addUltimatePlan() error {
	cmd := exec.Command("powercfg", "-duplicatescheme", guidUltimate)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.New(strings.TrimSpace(string(out)))
	}
	return nil
}

// bestPlan elige el mejor plan disponible: Máximo rendimiento > Alto rendimiento.
func bestPlan(preferred string) string {
	plans := powerPlans()
	if preferred != "" {
		for _, p := range plans {
			if p.GUID == preferred {
				return p.GUID
			}
		}
	}
	for _, k := range []string{"ultimate", "high"} {
		for _, p := range plans {
			if p.Kind == k {
				return p.GUID
			}
		}
	}
	for _, p := range plans {
		ln := strings.ToLower(p.Name)
		if strings.Contains(ln, "máximo") || strings.Contains(ln, "ultimate") || strings.Contains(ln, "alto") || strings.Contains(ln, "high") {
			return p.GUID
		}
	}
	return ""
}

type gameResult struct {
	Closed  int      `json:"closed"`
	Freed   uint64   `json:"freed"`
	Plan    string   `json:"plan"`
	Standby bool     `json:"standby"`
	Notes   []string `json:"notes"`
}

func gameModeOn() gameResult {
	var res gameResult
	c := getConfig()
	g := c.Game
	// 1) cerrar procesos
	if g.CloseBloat || len(g.Extra) > 0 {
		groups, _ := buildGroups(c)
		extra := map[string]bool{}
		for _, e := range g.Extra {
			extra[strings.ToLower(e)] = true
		}
		var targets []killTarget
		for _, gr := range groups {
			if gr.Protected || gr.Category == "app" && !extra[strings.ToLower(gr.Exe)] {
				continue
			}
			if (g.CloseBloat && gr.Auto) || extra[strings.ToLower(gr.Exe)] {
				for _, m := range gr.Members {
					targets = append(targets, killTarget{m.PID, m.CT})
				}
				res.Freed += gr.WS
			}
		}
		for _, r := range killMany(targets) {
			if r.OK {
				res.Closed++
			}
		}
	}
	// 2) plan de energía
	if g.Power {
		prev := activePlan()
		best := bestPlan(g.PlanGUID)
		if best == "" {
			res.Notes = append(res.Notes, "No hay ningún plan de alto rendimiento disponible. Puedes añadir «Máximo rendimiento» desde esta sección.")
		} else if best != prev {
			if err := setPlan(best); err == nil {
				res.Plan = best
				cfgMu.Lock()
				if cfg.GameActive == false {
					cfg.PrevPlan = prev
				}
				cfgMu.Unlock()
			} else {
				res.Notes = append(res.Notes, "No se pudo cambiar el plan de energía.")
			}
		} else {
			res.Plan = best
		}
	}
	// 3) memoria en espera
	if g.Standby {
		if !adminCached {
			res.Notes = append(res.Notes, "Liberar la memoria en espera requiere permisos de administrador.")
		} else if purgeStandby() == nil {
			res.Standby = true
		}
	}
	cfgMu.Lock()
	cfg.GameActive = true
	cfg.GameSince = time.Now().Unix()
	cfgMu.Unlock()
	saveConfig()
	return res
}

func gameModeOff() error {
	cfgMu.Lock()
	prev := cfg.PrevPlan
	cfg.GameActive = false
	cfg.PrevPlan = ""
	cfgMu.Unlock()
	saveConfig()
	if prev != "" {
		return setPlan(prev)
	}
	return nil
}
