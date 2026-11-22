//go:build windows

package main

// Información del sistema: Windows, placa base, BIOS, procesador, memoria,
// gráficas, almacenamiento, pantallas, red y batería.

import (
	"encoding/binary"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// ---------- SMBIOS ----------

type smbiosStruct struct {
	Type    byte
	Data    []byte
	Strings []string
}

func (s smbiosStruct) str(off int) string {
	if off >= len(s.Data) {
		return ""
	}
	i := int(s.Data[off])
	if i == 0 || i > len(s.Strings) {
		return ""
	}
	return strings.TrimSpace(s.Strings[i-1])
}
func (s smbiosStruct) u8(off int) byte {
	if off < len(s.Data) {
		return s.Data[off]
	}
	return 0
}
func (s smbiosStruct) u16(off int) uint16 {
	if off+2 <= len(s.Data) {
		return binary.LittleEndian.Uint16(s.Data[off:])
	}
	return 0
}
func (s smbiosStruct) u32(off int) uint32 {
	if off+4 <= len(s.Data) {
		return binary.LittleEndian.Uint32(s.Data[off:])
	}
	return 0
}

var (
	smbOnce sync.Once
	smbList []smbiosStruct
)

func smbios() []smbiosStruct {
	smbOnce.Do(func() {
		const rsmb = 0x52534D42
		n, _, _ := pGetFirmwareTable.Call(rsmb, 0, 0, 0)
		if n == 0 || n > 1<<20 {
			return
		}
		buf := make([]byte, n)
		pGetFirmwareTable.Call(rsmb, 0, uintptr(unsafe.Pointer(&buf[0])), n)
		if len(buf) < 8 {
			return
		}
		data := buf[8:]
		if l := int(binary.LittleEndian.Uint32(buf[4:])); l < len(data) {
			data = data[:l]
		}
		for i := 0; i+4 <= len(data); {
			t, l := data[i], int(data[i+1])
			if l < 4 || i+l > len(data) {
				break
			}
			st := smbiosStruct{Type: t, Data: data[i : i+l]}
			j := i + l
			for j < len(data) {
				k := j
				for k < len(data) && data[k] != 0 {
					k++
				}
				if k == j {
					j++
					break
				}
				st.Strings = append(st.Strings, string(data[j:k]))
				j = k + 1
			}
			if j < len(data) && len(st.Strings) == 0 && data[j] == 0 {
				j++
			}
			smbList = append(smbList, st)
			if t == 127 {
				break
			}
			i = j
		}
	})
	return smbList
}

type memModule struct {
	Slot  string `json:"slot"`
	Size  uint64 `json:"size"`
	Type  string `json:"type"`
	Speed int    `json:"speed"`
	Conf  int    `json:"confSpeed"`
	Maker string `json:"maker"`
	Part  string `json:"part"`
	Form  string `json:"form"`
}

var memTypes = map[byte]string{0x12: "DDR", 0x13: "DDR2", 0x18: "DDR3", 0x1A: "DDR4", 0x1B: "LPDDR", 0x1C: "LPDDR2", 0x1D: "LPDDR3", 0x1E: "LPDDR4", 0x22: "DDR5", 0x23: "LPDDR5"}
var formFactors = map[byte]string{0x09: "DIMM", 0x0D: "SODIMM", 0x0B: "RIMM", 0x0F: "FB-DIMM"}

func memModules() []memModule {
	var out []memModule
	for _, s := range smbios() {
		if s.Type != 17 {
			continue
		}
		sz := uint64(s.u16(0x0C))
		if sz == 0 || sz == 0xFFFF {
			continue
		}
		switch {
		case sz == 0x7FFF:
			sz = uint64(s.u32(0x1C)) << 20
		case sz&0x8000 != 0:
			sz = (sz & 0x7FFF) << 10
		default:
			sz <<= 20
		}
		m := memModule{Slot: s.str(0x10), Size: sz, Type: memTypes[s.u8(0x12)], Speed: int(s.u16(0x15)), Maker: s.str(0x17), Part: s.str(0x1A), Form: formFactors[s.u8(0x0E)]}
		if len(s.Data) >= 0x22 {
			m.Conf = int(s.u16(0x20))
		}
		if bl := s.str(0x11); bl != "" && !strings.Contains(m.Slot, bl) && len(bl) < 20 {
			m.Slot = strings.TrimSpace(bl + " " + m.Slot)
		}
		out = append(out, m)
	}
	return out
}

func memSlots() int {
	n := 0
	for _, s := range smbios() {
		if s.Type == 16 {
			n += int(s.u16(0x0D))
		}
	}
	return n
}

// ---------- procesador ----------

type cpuDesc struct {
	Name    string `json:"name"`
	Vendor  string `json:"vendor"`
	Cores   int    `json:"cores"`
	Threads int    `json:"threads"`
	BaseMHz int    `json:"baseMHz"`
	MaxMHz  int    `json:"maxMHz"`
	Socket  string `json:"socket"`
	L2      uint64 `json:"l2"`
	L3      uint64 `json:"l3"`
	Arch    string `json:"arch"`
}

var (
	cpuOnce sync.Once
	cpuData cpuDesc
)

func cpuInfo() cpuDesc {
	cpuOnce.Do(func() {
		c := cpuDesc{Threads: runtime.NumCPU(), Arch: runtime.GOARCH}
		if k, err := regOpen(HKLM, `HARDWARE\DESCRIPTION\System\CentralProcessor\0`, KEY_READ); err == nil {
			c.Name = strings.Join(strings.Fields(k.Str("ProcessorNameString")), " ")
			c.Vendor = k.Str("VendorIdentifier")
			if v, ok := k.Uint("~MHz"); ok {
				c.BaseMHz = int(v)
			}
			k.Close()
		}
		// núcleos físicos y cachés
		var n uint32
		pGetLogicalProcInfo.Call(0xFFFF, 0, uintptr(unsafe.Pointer(&n)))
		if n > 0 && n < 1<<22 {
			buf := make([]byte, n)
			if r, _, _ := pGetLogicalProcInfo.Call(0xFFFF, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n))); r != 0 {
				for off := 0; off+8 <= int(n); {
					rel := binary.LittleEndian.Uint32(buf[off:])
					size := int(binary.LittleEndian.Uint32(buf[off+4:]))
					if size <= 0 {
						break
					}
					switch rel {
					case 0:
						c.Cores++
					case 2:
						if off+16 <= len(buf) {
							level := buf[off+8]
							cs := uint64(binary.LittleEndian.Uint32(buf[off+12:]))
							if level == 2 {
								c.L2 += cs
							} else if level == 3 {
								c.L3 += cs
							}
						}
					}
					off += size
				}
			}
		}
		for _, s := range smbios() {
			if s.Type == 4 && s.u16(0x16) > 0 {
				c.Socket = s.str(0x04)
				c.MaxMHz = int(s.u16(0x14))
				if c.Name == "" {
					c.Name = s.str(0x10)
				}
				break
			}
		}
		cpuData = c
	})
	return cpuData
}

// ---------- gráficas (registro de controladores) ----------

type gpuDesc struct {
	Name   string `json:"name"`
	Vendor string `json:"vendor"`
	VRAM   uint64 `json:"vram"`
	Driver string `json:"driver"`
	Date   string `json:"date"`
}

var (
	gpuOnce sync.Once
	gpuData []gpuDesc
)

func gpuNames() []gpuDesc {
	gpuOnce.Do(func() {
		base := `SYSTEM\CurrentControlSet\Control\Class\{4d36e968-e325-11ce-bfc1-08002be10318}`
		k, err := regOpen(HKLM, base, KEY_READ)
		if err != nil {
			return
		}
		subs := k.Subkeys()
		k.Close()
		seen := map[string]bool{}
		for _, s := range subs {
			if len(s) != 4 {
				continue
			}
			sk, err := regOpen(HKLM, base+`\`+s, KEY_READ)
			if err != nil {
				continue
			}
			g := gpuDesc{Name: sk.Str("DriverDesc"), Vendor: sk.Str("ProviderName"), Driver: sk.Str("DriverVersion"), Date: sk.Str("DriverDate")}
			if v, ok := sk.Uint("HardwareInformation.qwMemorySize"); ok && v > 0 {
				g.VRAM = v
			} else if v, ok := sk.Uint("HardwareInformation.MemorySize"); ok {
				g.VRAM = v
			}
			sk.Close()
			ln := strings.ToLower(g.Name)
			if g.Name == "" || seen[g.Name] || strings.Contains(ln, "basic display") || strings.Contains(ln, "basic render") || strings.Contains(ln, "virtual") || strings.Contains(ln, "parsec") || strings.Contains(ln, "remote") {
				continue
			}
			seen[g.Name] = true
			gpuData = append(gpuData, g)
		}
	})
	return gpuData
}

// ---------- pantallas ----------

type displayDesc struct {
	Name    string `json:"name"`
	Adapter string `json:"adapter"`
	Width   uint32 `json:"w"`
	Height  uint32 `json:"h"`
	Hz      uint32 `json:"hz"`
	Bits    uint32 `json:"bits"`
	Primary bool   `json:"primary"`
}

func displays() []displayDesc {
	var out []displayDesc
	for i := 0; i < 16; i++ {
		var dd [840]byte
		binary.LittleEndian.PutUint32(dd[:], 840)
		if r, _, _ := pEnumDisplayDevicesW.Call(0, uintptr(i), uintptr(unsafe.Pointer(&dd[0])), 0); r == 0 {
			break
		}
		flags := binary.LittleEndian.Uint32(dd[324:])
		if flags&1 == 0 {
			continue
		}
		devName := utf16Bytes(dd[4:68])
		d := displayDesc{Adapter: utf16Bytes(dd[68:324]), Primary: flags&4 != 0}
		var mon [840]byte
		binary.LittleEndian.PutUint32(mon[:], 840)
		if r, _, _ := pEnumDisplayDevicesW.Call(uintptr(unsafe.Pointer(u16(devName))), 0, uintptr(unsafe.Pointer(&mon[0])), 0); r != 0 {
			d.Name = utf16Bytes(mon[68:324])
		}
		var dm [220]byte
		binary.LittleEndian.PutUint16(dm[68:], 220)
		if r, _, _ := pEnumDisplaySettings.Call(uintptr(unsafe.Pointer(u16(devName))), ^uintptr(0), uintptr(unsafe.Pointer(&dm[0]))); r != 0 {
			d.Bits = binary.LittleEndian.Uint32(dm[168:])
			d.Width = binary.LittleEndian.Uint32(dm[172:])
			d.Height = binary.LittleEndian.Uint32(dm[176:])
			d.Hz = binary.LittleEndian.Uint32(dm[184:])
		}
		out = append(out, d)
	}
	return out
}

// ---------- informe completo ----------

func windowsInfo() map[string]any {
	info := map[string]any{}
	var vi [284]byte
	binary.LittleEndian.PutUint32(vi[:], 284)
	pRtlGetVersion.Call(uintptr(unsafe.Pointer(&vi[0])))
	major, minor, build := binary.LittleEndian.Uint32(vi[4:]), binary.LittleEndian.Uint32(vi[8:]), binary.LittleEndian.Uint32(vi[12:])
	name := "Windows"
	if k, err := regOpen(HKLM, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, KEY_READ|KEY_WOW64_64KEY); err == nil {
		name = k.Str("ProductName")
		info["displayVersion"] = k.Str("DisplayVersion")
		info["edition"] = k.Str("EditionID")
		ubr, _ := k.Uint("UBR")
		info["build"] = itoa(int64(build)) + "." + itoa(int64(ubr))
		if v, ok := k.Uint("InstallDate"); ok {
			info["installDate"] = int64(v)
		}
		info["owner"] = k.Str("RegisteredOwner")
		k.Close()
	}
	if build >= 22000 {
		name = strings.Replace(name, "Windows 10", "Windows 11", 1)
	}
	info["name"] = name
	info["version"] = itoa(int64(major)) + "." + itoa(int64(minor))
	host, _ := os.Hostname()
	info["computer"] = host
	info["user"] = os.Getenv("USERNAME")
	info["arch"] = os.Getenv("PROCESSOR_ARCHITECTURE")
	info["bootTime"] = time.Now().Add(-time.Duration(tick64()) * time.Millisecond).Unix()
	return info
}

func systemReport() map[string]any {
	rep := map[string]any{"os": windowsInfo(), "cpu": cpuInfo(), "gpus": gpuNames(), "displays": displays()}
	board := map[string]string{}
	for _, s := range smbios() {
		switch s.Type {
		case 0:
			board["biosVendor"], board["biosVersion"], board["biosDate"] = s.str(4), s.str(5), s.str(8)
		case 1:
			board["sysMaker"], board["sysModel"] = s.str(4), s.str(5)
		case 2:
			board["boardMaker"], board["boardModel"], board["boardVersion"] = s.str(4), s.str(5), s.str(6)
		}
	}
	rep["board"] = board
	ms := memStatus()
	rep["memory"] = map[string]any{"total": ms.TotalPhys, "installed": installedMemory(), "modules": memModules(), "slots": memSlots()}
	rep["disks"] = physicalDisks()
	var vols []driveInfo
	for _, d := range listDrives() {
		if d.Total > 0 {
			vols = append(vols, d)
		}
	}
	rep["volumes"] = vols
	// red
	type nic struct {
		Name  string   `json:"name"`
		MAC   string   `json:"mac"`
		Addrs []string `json:"addrs"`
		Up    bool     `json:"up"`
	}
	var nics []nic
	ifs, _ := net.Interfaces()
	for _, it := range ifs {
		if it.Flags&net.FlagLoopback != 0 {
			continue
		}
		n := nic{Name: it.Name, MAC: it.HardwareAddr.String(), Up: it.Flags&net.FlagUp != 0}
		addrs, _ := it.Addrs()
		for _, a := range addrs {
			s := a.String()
			if strings.HasPrefix(s, "fe80:") || strings.HasPrefix(s, "169.254.") {
				continue
			}
			n.Addrs = append(n.Addrs, s)
		}
		if n.MAC == "" && len(n.Addrs) == 0 {
			continue
		}
		nics = append(nics, n)
	}
	rep["network"] = nics
	// batería
	var ps [12]byte
	if r, _, _ := pGetSystemPowerStat.Call(uintptr(unsafe.Pointer(&ps[0]))); r != 0 && ps[1] != 128 && ps[1] != 255 {
		rep["battery"] = map[string]any{"percent": ps[2], "ac": ps[0] == 1, "charging": ps[1]&8 != 0, "seconds": int32(binary.LittleEndian.Uint32(ps[4:]))}
	}
	_ = syscall.Getpid
	return rep
}
