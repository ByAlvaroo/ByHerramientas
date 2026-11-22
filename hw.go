//go:build windows

package main

// Monitor de hardware: procesador, memoria, gráfica (NVIDIA mediante NVML),
// discos (actividad y temperatura) y sensores de LibreHardwareMonitor si está abierto.

import (
	"encoding/binary"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	pCreateFileW         = kernel32.NewProc("CreateFileW")
	pDeviceIoControl     = kernel32.NewProc("DeviceIoControl")
	pGetPerformanceInfo  = kernel32.NewProc("K32GetPerformanceInfo")
	pGetFirmwareTable    = kernel32.NewProc("GetSystemFirmwareTable")
	pGetInstalledMemory  = kernel32.NewProc("GetPhysicallyInstalledSystemMemory")
	pGetLogicalProcInfo  = kernel32.NewProc("GetLogicalProcessorInformationEx")
	pGetSystemPowerStat  = kernel32.NewProc("GetSystemPowerStatus")
	pRtlGetVersion       = ntdll.NewProc("RtlGetVersion")
	pNtSetSystemInfo     = ntdll.NewProc("NtSetSystemInformation")
	pEnumDisplayDevicesW = user32.NewProc("EnumDisplayDevicesW")
	pEnumDisplaySettings = user32.NewProc("EnumDisplaySettingsW")
)

// ---------- dispositivos de almacenamiento ----------

func openDevice(path string, access uint32) (syscall.Handle, error) {
	h, _, e := pCreateFileW.Call(uintptr(unsafe.Pointer(u16(path))), uintptr(access), 3 /*share r|w*/, 0, 3 /*OPEN_EXISTING*/, 0, 0)
	if syscall.Handle(h) == syscall.InvalidHandle {
		return syscall.InvalidHandle, e
	}
	return syscall.Handle(h), nil
}

func ioctl(h syscall.Handle, code uint32, in []byte, outSize int) ([]byte, bool) {
	out := make([]byte, outSize)
	var ret uint32
	var ip uintptr
	if len(in) > 0 {
		ip = uintptr(unsafe.Pointer(&in[0]))
	}
	r, _, _ := pDeviceIoControl.Call(uintptr(h), uintptr(code), ip, uintptr(len(in)), uintptr(unsafe.Pointer(&out[0])), uintptr(outSize), uintptr(unsafe.Pointer(&ret)), 0)
	if r == 0 {
		return nil, false
	}
	return out[:ret], true
}

const ioctlStorageQueryProperty = 0x2D1400

func storageQuery(h syscall.Handle, prop uint32, outSize int) ([]byte, bool) {
	q := make([]byte, 12)
	binary.LittleEndian.PutUint32(q[0:], prop)
	return ioctl(h, ioctlStorageQueryProperty, q, outSize)
}

type physDisk struct {
	Index   int      `json:"index"`
	Model   string   `json:"model"`
	Serial  string   `json:"serial,omitempty"`
	Bus     string   `json:"bus"`
	Kind    string   `json:"kind"` // SSD | HDD | ""
	Size    uint64   `json:"size"`
	Temp    float64  `json:"temp"`
	TempMax float64  `json:"tempWarn"`
	Removab bool     `json:"removable"`
	Volumes []string `json:"volumes"`
	Active  float64  `json:"active"`
	Read    float64  `json:"read"`
	Write   float64  `json:"write"`
}

var busNames = map[uint32]string{1: "SCSI", 2: "ATAPI", 3: "ATA", 4: "1394", 6: "Fibre", 7: "USB", 8: "RAID", 9: "iSCSI", 10: "SAS", 11: "SATA", 12: "SD", 13: "MMC", 14: "Virtual", 15: "Virtual", 16: "Espacios", 17: "NVMe", 18: "SCM", 19: "UFS"}

func cstr(b []byte, off uint32) string {
	if off == 0 || int(off) >= len(b) {
		return ""
	}
	e := int(off)
	for e < len(b) && b[e] != 0 {
		e++
	}
	return strings.TrimSpace(string(b[off:e]))
}

var (
	diskCacheMu sync.Mutex
	diskCache   []physDisk
	diskCacheAt time.Time
)

// physicalDisks lista los discos físicos (información estática cacheada 30 s; temperatura en cada llamada).
func physicalDisks() []physDisk {
	diskCacheMu.Lock()
	defer diskCacheMu.Unlock()
	if diskCache != nil && time.Since(diskCacheAt) < 30*time.Second {
		out := append([]physDisk(nil), diskCache...)
		for i := range out {
			out[i].Temp, out[i].TempMax = diskTemp(out[i].Index)
		}
		return out
	}
	var out []physDisk
	for i := 0; i < 16; i++ {
		h, err := openDevice(`\\.\PhysicalDrive`+itoa(int64(i)), 0)
		if err != nil {
			continue
		}
		d := physDisk{Index: i}
		if b, ok := storageQuery(h, 0, 1024); ok && len(b) >= 36 {
			vendor := cstr(b, binary.LittleEndian.Uint32(b[12:]))
			product := cstr(b, binary.LittleEndian.Uint32(b[16:]))
			d.Serial = cstr(b, binary.LittleEndian.Uint32(b[24:]))
			d.Model = strings.TrimSpace(vendor + " " + product)
			if vendor != "" && strings.HasPrefix(strings.ToLower(product), strings.ToLower(vendor)) {
				d.Model = product
			}
			d.Bus = busNames[binary.LittleEndian.Uint32(b[28:])]
			d.Removab = b[10] != 0
		}
		if b, ok := storageQuery(h, 7, 12); ok && len(b) >= 9 {
			if b[8] == 0 {
				d.Kind = "SSD"
			} else {
				d.Kind = "HDD"
			}
		}
		if b, ok := ioctl(h, 0x700A0, nil, 256); ok && len(b) >= 32 {
			d.Size = binary.LittleEndian.Uint64(b[24:])
		}
		syscall.CloseHandle(h)
		d.Temp, d.TempMax = diskTemp(i)
		out = append(out, d)
	}
	// unidades (C:, D:…) de cada disco
	for _, dr := range listDrives() {
		if dr.Kind != "fixed" && dr.Kind != "removable" {
			continue
		}
		h, err := openDevice(`\\.\`+strings.TrimRight(dr.Path, `\`), 0)
		if err != nil {
			continue
		}
		if b, ok := ioctl(h, 0x560000, nil, 256); ok && len(b) >= 12 && binary.LittleEndian.Uint32(b) > 0 {
			n := int(binary.LittleEndian.Uint32(b[8:]))
			for j := range out {
				if out[j].Index == n {
					out[j].Volumes = append(out[j].Volumes, strings.TrimRight(dr.Path, `\`))
				}
			}
		}
		syscall.CloseHandle(h)
	}
	diskCache = out
	diskCacheAt = time.Now()
	return append([]physDisk(nil), out...)
}

func diskTemp(i int) (float64, float64) {
	for _, acc := range []uint32{0, 0x80000000} {
		h, err := openDevice(`\\.\PhysicalDrive`+itoa(int64(i)), acc)
		if err != nil {
			continue
		}
		b, ok := storageQuery(h, 10, 512) // StorageDeviceTemperatureProperty
		syscall.CloseHandle(h)
		if ok && len(b) >= 28 && binary.LittleEndian.Uint16(b[12:]) > 0 {
			t := int16(binary.LittleEndian.Uint16(b[26:]))
			warn := int16(binary.LittleEndian.Uint16(b[10:]))
			if t > 0 && t < 120 {
				return float64(t), float64(warn)
			}
		}
	}
	return 0, 0
}

// ---------- NVIDIA (NVML, incluido en el controlador) ----------

type nvGPU struct {
	Name     string  `json:"name"`
	Temp     float64 `json:"temp"`
	Util     float64 `json:"util"`
	MemUtil  float64 `json:"memUtil"`
	MemUsed  uint64  `json:"memUsed"`
	MemTotal uint64  `json:"memTotal"`
	Power    float64 `json:"power"`
	PowerMax float64 `json:"powerMax"`
	Clock    uint32  `json:"clock"`
	MemClock uint32  `json:"memClock"`
	Fan      uint32  `json:"fan"`
	Driver   string  `json:"driver"`
}

var (
	nvOnce sync.Once
	nvDLL  *syscall.LazyDLL
	nvOK   bool
	nvMu   sync.Mutex
)

func nvInit() {
	nvOnce.Do(func() {
		for _, p := range []string{
			filepath.Join(os.Getenv("SystemRoot"), "System32", "nvml.dll"),
			filepath.Join(os.Getenv("ProgramFiles"), "NVIDIA Corporation", "NVSMI", "nvml.dll"),
		} {
			if _, err := os.Stat(p); err != nil {
				continue
			}
			d := syscall.NewLazyDLL(p)
			if d.Load() != nil {
				continue
			}
			if r, _, _ := d.NewProc("nvmlInit_v2").Call(); r == 0 {
				nvDLL, nvOK = d, true
				return
			}
		}
	})
}

func nvidiaGPUs() []nvGPU {
	nvInit()
	if !nvOK {
		return nil
	}
	nvMu.Lock()
	defer nvMu.Unlock()
	call := func(name string, a ...uintptr) bool {
		p := nvDLL.NewProc(name)
		if p.Find() != nil {
			return false
		}
		r, _, _ := p.Call(a...)
		return r == 0
	}
	var drv [96]byte
	call("nvmlSystemGetDriverVersion", uintptr(unsafe.Pointer(&drv[0])), 96)
	var n uint32
	if !call("nvmlDeviceGetCount_v2", uintptr(unsafe.Pointer(&n))) {
		return nil
	}
	var out []nvGPU
	for i := uint32(0); i < n && i < 8; i++ {
		var h uintptr
		if !call("nvmlDeviceGetHandleByIndex_v2", uintptr(i), uintptr(unsafe.Pointer(&h))) {
			continue
		}
		g := nvGPU{Driver: cstr(drv[:], 0)}
		var name [96]byte
		if call("nvmlDeviceGetName", h, uintptr(unsafe.Pointer(&name[0])), 96) {
			g.Name = cstr(name[:], 0)
		}
		var t uint32
		if call("nvmlDeviceGetTemperature", h, 0, uintptr(unsafe.Pointer(&t))) {
			g.Temp = float64(t)
		}
		var u struct{ GPU, Mem uint32 }
		if call("nvmlDeviceGetUtilizationRates", h, uintptr(unsafe.Pointer(&u))) {
			g.Util, g.MemUtil = float64(u.GPU), float64(u.Mem)
		}
		var m struct{ Total, Free, Used uint64 }
		if call("nvmlDeviceGetMemoryInfo", h, uintptr(unsafe.Pointer(&m))) {
			g.MemTotal, g.MemUsed = m.Total, m.Used
		}
		var pw uint32
		if call("nvmlDeviceGetPowerUsage", h, uintptr(unsafe.Pointer(&pw))) {
			g.Power = float64(pw) / 1000
		}
		if call("nvmlDeviceGetEnforcedPowerLimit", h, uintptr(unsafe.Pointer(&pw))) {
			g.PowerMax = float64(pw) / 1000
		}
		call("nvmlDeviceGetClockInfo", h, 0, uintptr(unsafe.Pointer(&g.Clock)))
		call("nvmlDeviceGetClockInfo", h, 2, uintptr(unsafe.Pointer(&g.MemClock)))
		call("nvmlDeviceGetFanSpeed", h, uintptr(unsafe.Pointer(&g.Fan)))
		out = append(out, g)
	}
	return out
}

// ---------- LibreHardwareMonitor (servidor web opcional en el puerto 8085) ----------

type lhmSensor struct {
	Hardware string `json:"hw"`
	Group    string `json:"group"`
	Name     string `json:"name"`
	Value    string `json:"value"`
	Min      string `json:"min"`
	Max      string `json:"max"`
}

var (
	lhmMu   sync.Mutex
	lhmLast time.Time
	lhmData []lhmSensor
	lhmPort = 8085
)

func lhmSensors() []lhmSensor {
	lhmMu.Lock()
	defer lhmMu.Unlock()
	if time.Since(lhmLast) < 1500*time.Millisecond {
		return lhmData
	}
	lhmLast = time.Now()
	c := http.Client{Timeout: 350 * time.Millisecond}
	r, err := c.Get("http://127.0.0.1:" + strconv.Itoa(lhmPort) + "/data.json")
	if err != nil {
		lhmData = nil
		return nil
	}
	defer r.Body.Close()
	type node struct {
		Text     string `json:"Text"`
		Value    string `json:"Value"`
		Min      string `json:"Min"`
		Max      string `json:"Max"`
		ImageURL string `json:"ImageURL"`
		Children []node `json:"Children"`
	}
	var root node
	if json.NewDecoder(r.Body).Decode(&root) != nil {
		lhmData = nil
		return nil
	}
	var out []lhmSensor
	isGroup := func(n node) bool {
		if len(n.Children) == 0 {
			return false
		}
		for _, c := range n.Children {
			if len(c.Children) > 0 {
				return false
			}
		}
		return true
	}
	var walk func(n node, hw string, depth int)
	walk = func(n node, hw string, depth int) {
		if isGroup(n) {
			for _, c := range n.Children {
				if c.Value != "" {
					out = append(out, lhmSensor{hw, n.Text, c.Text, c.Value, c.Min, c.Max})
				}
			}
			return
		}
		for _, ch := range n.Children {
			h := hw
			if depth >= 1 && !isGroup(ch) {
				if hw == "" {
					h = ch.Text
				} else {
					h = hw + " › " + ch.Text
				}
			}
			walk(ch, h, depth+1)
		}
	}
	walk(root, "", 0)
	lhmData = out
	return out
}

// ---------- memoria ----------

type perfInfo struct {
	Cb                uint32
	CommitTotal       uintptr
	CommitLimit       uintptr
	CommitPeak        uintptr
	PhysicalTotal     uintptr
	PhysicalAvailable uintptr
	SystemCache       uintptr
	KernelTotal       uintptr
	KernelPaged       uintptr
	KernelNonpaged    uintptr
	PageSize          uintptr
	HandleCount       uint32
	ProcessCount      uint32
	ThreadCount       uint32
}

func performanceInfo() perfInfo {
	var p perfInfo
	p.Cb = uint32(unsafe.Sizeof(p))
	pGetPerformanceInfo.Call(uintptr(unsafe.Pointer(&p)), uintptr(p.Cb))
	return p
}

// purgeStandby vacía la lista de memoria en espera (requiere administrador).
func purgeStandby() error {
	if err := enablePrivilege("SeProfileSingleProcessPrivilege"); err != nil {
		return err
	}
	cmd := uint32(4) // MemoryPurgeStandbyList
	r, _, _ := pNtSetSystemInfo.Call(80, uintptr(unsafe.Pointer(&cmd)), 4)
	if r != 0 {
		return syscall.Errno(r)
	}
	return nil
}

func enablePrivilege(name string) error {
	var tok syscall.Token
	proc, _ := syscall.GetCurrentProcess()
	if err := syscall.OpenProcessToken(proc, 0x20|0x8, &tok); err != nil { // ADJUST_PRIVILEGES | QUERY
		return err
	}
	defer tok.Close()
	var luid struct{ Low, High uint32 }
	r, _, e := advapi32.NewProc("LookupPrivilegeValueW").Call(0, uintptr(unsafe.Pointer(u16(name))), uintptr(unsafe.Pointer(&luid)))
	if r == 0 {
		return e
	}
	tp := struct {
		Count uint32
		Low   uint32
		High  uint32
		Attr  uint32
	}{1, luid.Low, luid.High, 2}
	r, _, e = advapi32.NewProc("AdjustTokenPrivileges").Call(uintptr(tok), 0, uintptr(unsafe.Pointer(&tp)), 0, 0, 0)
	if r == 0 {
		return e
	}
	if e == syscall.Errno(1300) { // ERROR_NOT_ALL_ASSIGNED
		return e
	}
	return nil
}

// ---------- instantánea completa para la sección Hardware ----------

func hardwareSnapshot() map[string]any {
	perf.collect()
	_, cpu := smp.get(900 * time.Millisecond)

	// núcleos
	cores := perf.array("cpuCore")
	if len(cores) == 0 {
		cores = perf.array("cpuCoreT")
	}
	type core struct {
		ID  string  `json:"id"`
		Val float64 `json:"v"`
	}
	var cl []core
	for k, v := range cores {
		if strings.Contains(k, "_Total") {
			continue
		}
		if v > 100 {
			v = 100
		}
		cl = append(cl, core{k, v})
	}
	sort.Slice(cl, func(i, j int) bool { return coreLess(cl[i].ID, cl[j].ID) })
	freq, _ := perf.value("cpuFreq")
	perfPct, _ := perf.value("cpuPerf")
	curMHz := freq * perfPct / 100

	// zonas térmicas ACPI (décimas de kelvin)
	var thermal []map[string]any
	tz := perf.array("thermal")
	scale := 10.0
	if len(tz) == 0 {
		tz = perf.array("thermalK")
		scale = 1
	}
	for k, v := range tz {
		c := v/scale - 273.15
		if c > 0 && c < 125 {
			thermal = append(thermal, map[string]any{"name": k, "temp": c})
		}
	}

	// memoria
	ms := memStatus()
	pi := performanceInfo()
	pv := func(k string) float64 { v, _ := perf.value(k); return v }
	standby := pv("memStbN") + pv("memStbR") + pv("memStbC")

	// gráficas
	overall, byType, byLuid := gpuUsage()
	ded := perf.array("gpuDed")
	shared := perf.array("gpuShared")
	type gpuAd struct {
		Luid   string  `json:"luid"`
		Util   float64 `json:"util"`
		Ded    float64 `json:"ded"`
		Shared float64 `json:"shared"`
	}
	var adapters []gpuAd
	for l, u := range byLuid {
		a := gpuAd{Luid: l, Util: u}
		for k, v := range ded {
			if strings.HasPrefix(k, l) {
				a.Ded += v
			}
		}
		for k, v := range shared {
			if strings.HasPrefix(k, l) {
				a.Shared += v
			}
		}
		adapters = append(adapters, a)
	}
	sort.Slice(adapters, func(i, j int) bool { return adapters[i].Ded > adapters[j].Ded })

	// discos
	disks := physicalDisks()
	idle := perf.array("diskIdle")
	rd := perf.array("diskRead")
	wr := perf.array("diskWrite")
	for i := range disks {
		prefix := itoa(int64(disks[i].Index)) + " "
		for k, v := range idle {
			if strings.HasPrefix(k+" ", prefix) {
				a := 100 - v
				if a < 0 {
					a = 0
				}
				disks[i].Active = a
				disks[i].Read = rd[k]
				disks[i].Write = wr[k]
			}
		}
	}

	return map[string]any{
		"cpu": map[string]any{"usage": cpu, "cores": cl, "mhz": curMHz, "baseMHz": freq, "info": cpuInfo(),
			"queue": pv("procQueue"), "ctx": pv("ctxSwitch"), "handles": pi.HandleCount, "threads": pi.ThreadCount, "procs": pi.ProcessCount, "uptime": tick64() / 1000},
		"thermal": thermal,
		"mem": map[string]any{"total": ms.TotalPhys, "avail": ms.AvailPhys, "load": ms.MemoryLoad,
			"standby": standby, "modified": pv("memMod"), "free": pv("memFree"), "cache": uint64(pi.SystemCache) * uint64(pi.PageSize),
			"commit": uint64(pi.CommitTotal) * uint64(pi.PageSize), "commitLimit": uint64(pi.CommitLimit) * uint64(pi.PageSize),
			"paged": pv("memPaged"), "nonpaged": pv("memNP"), "faults": pv("memFaults"), "modules": memModules(), "installed": installedMemory()},
		"gpu":   map[string]any{"util": overall, "engines": byType, "adapters": adapters, "nvidia": nvidiaGPUs(), "names": gpuNames()},
		"disks": disks,
		"net":   netTotals(),
		"lhm":   lhmSensors(),
		"admin": adminCached,
	}
}

func coreLess(a, b string) bool {
	pa, pb := strings.Split(a, ","), strings.Split(b, ",")
	for i := 0; i < len(pa) && i < len(pb); i++ {
		x, e1 := strconv.Atoi(pa[i])
		y, e2 := strconv.Atoi(pb[i])
		if e1 == nil && e2 == nil && x != y {
			return x < y
		}
	}
	return a < b
}

func installedMemory() uint64 {
	var kb uint64
	if r, _, _ := pGetInstalledMemory.Call(uintptr(unsafe.Pointer(&kb))); r != 0 {
		return kb * 1024
	}
	return 0
}
