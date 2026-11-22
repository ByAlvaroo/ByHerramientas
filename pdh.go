//go:build windows

package main

// Contadores de rendimiento de Windows (PDH): uso por núcleo, frecuencia, discos,
// GPU y zonas térmicas. Son los mismos datos que usa el Administrador de tareas.

import (
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	pdhDLL                       = syscall.NewLazyDLL("pdh.dll")
	pPdhOpenQueryW               = pdhDLL.NewProc("PdhOpenQueryW")
	pPdhAddEnglishCounterW       = pdhDLL.NewProc("PdhAddEnglishCounterW")
	pPdhCollectQueryData         = pdhDLL.NewProc("PdhCollectQueryData")
	pPdhGetFormattedCounterValue = pdhDLL.NewProc("PdhGetFormattedCounterValue")
	pPdhGetFormattedCounterArray = pdhDLL.NewProc("PdhGetFormattedCounterArrayW")
)

const (
	pdhFmtDouble   = 0x00000200
	pdhFmtNoCap100 = 0x00008000
	pdhMoreData    = 0x800007D2
)

type pdhQuery struct {
	mu       sync.Mutex
	q        uintptr
	counters map[string]uintptr
	last     time.Time
	ok       bool
}

var perf = &pdhQuery{counters: map[string]uintptr{}}

var perfPaths = map[string]string{
	"cpuCore":   `\Processor Information(*)\% Processor Utility`,
	"cpuCoreT":  `\Processor Information(*)\% Processor Time`,
	"cpuPerf":   `\Processor Information(_Total)\% Processor Performance`,
	"cpuFreq":   `\Processor Information(_Total)\Processor Frequency`,
	"diskIdle":  `\PhysicalDisk(*)\% Idle Time`,
	"diskRead":  `\PhysicalDisk(*)\Disk Read Bytes/sec`,
	"diskWrite": `\PhysicalDisk(*)\Disk Write Bytes/sec`,
	"gpuEngine": `\GPU Engine(*)\Utilization Percentage`,
	"gpuDed":    `\GPU Adapter Memory(*)\Dedicated Usage`,
	"gpuShared": `\GPU Adapter Memory(*)\Shared Usage`,
	"thermal":   `\Thermal Zone Information(*)\High Precision Temperature`,
	"thermalK":  `\Thermal Zone Information(*)\Temperature`,
	"memCache":  `\Memory\Cache Bytes`,
	"memStbN":   `\Memory\Standby Cache Normal Priority Bytes`,
	"memStbR":   `\Memory\Standby Cache Reserve Bytes`,
	"memStbC":   `\Memory\Standby Cache Core Bytes`,
	"memMod":    `\Memory\Modified Page List Bytes`,
	"memFree":   `\Memory\Free & Zero Page List Bytes`,
	"memPaged":  `\Memory\Pool Paged Bytes`,
	"memNP":     `\Memory\Pool Nonpaged Bytes`,
	"memComp":   `\Memory\Committed Bytes`,
	"memLimit":  `\Memory\Commit Limit`,
	"memFaults": `\Memory\Page Faults/sec`,
	"procQueue": `\System\Processor Queue Length`,
	"ctxSwitch": `\System\Context Switches/sec`,
	"handles":   `\Process(_Total)\Handle Count`,
}

func (p *pdhQuery) init() {
	if p.q != 0 || pdhDLL.Load() != nil {
		return
	}
	var q uintptr
	if r, _, _ := pPdhOpenQueryW.Call(0, 0, uintptr(unsafe.Pointer(&q))); r != 0 {
		return
	}
	p.q = q
	for k, path := range perfPaths {
		var c uintptr
		if r, _, _ := pPdhAddEnglishCounterW.Call(q, uintptr(unsafe.Pointer(u16(path))), 0, uintptr(unsafe.Pointer(&c))); r == 0 {
			p.counters[k] = c
		}
	}
	pPdhCollectQueryData.Call(q)
	p.last = time.Now()
	p.ok = true
}

// collect actualiza los contadores (como mucho cada 800 ms).
func (p *pdhQuery) collect() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.init()
	if !p.ok {
		return false
	}
	if time.Since(p.last) < 800*time.Millisecond {
		return true
	}
	pPdhCollectQueryData.Call(p.q)
	p.last = time.Now()
	return true
}

func (p *pdhQuery) value(key string) (float64, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	c, ok := p.counters[key]
	if !ok {
		return 0, false
	}
	var typ uint32
	var v struct {
		Status uint32
		_      uint32
		Val    float64
	}
	r, _, _ := pPdhGetFormattedCounterValue.Call(c, pdhFmtDouble|pdhFmtNoCap100, uintptr(unsafe.Pointer(&typ)), uintptr(unsafe.Pointer(&v)))
	if r != 0 || v.Status > 1 {
		return 0, false
	}
	return v.Val, true
}

// array devuelve los valores de un contador con comodín (*) por instancia.
func (p *pdhQuery) array(key string) map[string]float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := map[string]float64{}
	c, ok := p.counters[key]
	if !ok {
		return out
	}
	var size, count uint32
	r, _, _ := pPdhGetFormattedCounterArray.Call(c, pdhFmtDouble|pdhFmtNoCap100, uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&count)), 0)
	if uint32(r) != pdhMoreData || size == 0 {
		return out
	}
	buf := make([]byte, size+64)
	r, _, _ = pPdhGetFormattedCounterArray.Call(c, pdhFmtDouble|pdhFmtNoCap100, uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&buf[0])))
	if r != 0 {
		return out
	}
	type item struct {
		Name   *uint16
		Status uint32
		_      uint32
		Val    float64
	}
	items := unsafe.Slice((*item)(unsafe.Pointer(&buf[0])), count)
	for _, it := range items {
		if it.Name == nil || it.Status > 1 {
			continue
		}
		name := syscall.UTF16ToString(unsafe.Slice(it.Name, 512))
		out[name] += it.Val
	}
	return out
}

// gpuUsage agrega el uso por tipo de motor (3D, copia, vídeo…) como hace el Administrador de tareas.
func gpuUsage() (overall float64, byType map[string]float64, byLuid map[string]float64) {
	engines := perf.array("gpuEngine")
	perEngine := map[string]float64{} // luid_phys_eng -> suma de todos los procesos
	engType := map[string]string{}
	engLuid := map[string]string{}
	for name, v := range engines {
		i := strings.Index(name, "luid_")
		j := strings.Index(name, "_engtype_")
		if i < 0 || j < 0 || j < i {
			continue
		}
		key := name[i:j]
		perEngine[key] += v
		engType[key] = name[j+9:]
		parts := strings.SplitN(name[i:], "_phys_", 2)
		engLuid[key] = parts[0]
	}
	byType = map[string]float64{}
	byLuid = map[string]float64{}
	for k, v := range perEngine {
		if v > 100 {
			v = 100
		}
		t := engType[k]
		if v > byType[t] {
			byType[t] = v
		}
		if v > byLuid[engLuid[k]] {
			byLuid[engLuid[k]] = v
		}
		if v > overall {
			overall = v
		}
	}
	return
}
