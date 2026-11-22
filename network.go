//go:build windows

package main

// Monitor de red: velocidad total por adaptador, conexiones por programa y,
// con permisos de administrador, tráfico TCP por programa (estadísticas extendidas de Windows).

import (
	"encoding/binary"
	"net"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	iphlp                = syscall.NewLazyDLL("iphlpapi.dll")
	pGetIfTable2         = iphlp.NewProc("GetIfTable2")
	pFreeMibTable        = iphlp.NewProc("FreeMibTable")
	pGetExtendedTcpTable = iphlp.NewProc("GetExtendedTcpTable")
	pGetExtendedUdpTable = iphlp.NewProc("GetExtendedUdpTable")
	pSetPerTcpEStats     = iphlp.NewProc("SetPerTcpConnectionEStats")
	pGetPerTcpEStats     = iphlp.NewProc("GetPerTcpConnectionEStats")
	pSetPerTcp6EStats    = iphlp.NewProc("SetPerTcp6ConnectionEStats")
	pGetPerTcp6EStats    = iphlp.NewProc("GetPerTcp6ConnectionEStats")
)

// ---------- adaptadores ----------

type ifaceStat struct {
	Name    string  `json:"name"`
	Desc    string  `json:"desc"`
	Speed   uint64  `json:"speed"`
	In      uint64  `json:"in"`
	Out     uint64  `json:"out"`
	InRate  float64 `json:"inRate"`
	OutRate float64 `json:"outRate"`
	Type    uint32  `json:"type"`
	MAC     string  `json:"mac"`
}

var (
	ifMu   sync.Mutex
	ifPrev = map[uint64][2]uint64{}
	ifAt   time.Time
	ifLast []ifaceStat
)

func netTotals() map[string]any {
	ifMu.Lock()
	defer ifMu.Unlock()
	if ifLast != nil && time.Since(ifAt) < 800*time.Millisecond {
		return summarizeIfaces(ifLast)
	}
	var tbl uintptr
	if r, _, _ := pGetIfTable2.Call(uintptr(unsafe.Pointer(&tbl))); r != 0 || tbl == 0 {
		return map[string]any{}
	}
	defer pFreeMibTable.Call(tbl)
	n := *(*uint32)(unsafe.Pointer(tbl))
	const rowSize = 1352
	rows := unsafe.Slice((*byte)(unsafe.Pointer(tbl+8)), int(n)*rowSize)
	now := time.Now()
	el := now.Sub(ifAt).Seconds()
	next := map[uint64][2]uint64{}
	var out []ifaceStat
	for i := 0; i < int(n); i++ {
		r := rows[i*rowSize : (i+1)*rowSize]
		flags := r[1152]
		oper := binary.LittleEndian.Uint32(r[1156:])
		typ := binary.LittleEndian.Uint32(r[1128:])
		if flags&1 == 0 || oper != 1 || typ == 24 { // solo hardware, conectado, sin loopback
			continue
		}
		luid := binary.LittleEndian.Uint64(r[0:])
		s := ifaceStat{
			Name: utf16Bytes(r[28:542]), Desc: utf16Bytes(r[542:1056]), Type: typ,
			Speed: binary.LittleEndian.Uint64(r[1200:]),
			In:    binary.LittleEndian.Uint64(r[1208:]), Out: binary.LittleEndian.Uint64(r[1280:]),
		}
		if l := binary.LittleEndian.Uint32(r[1056:]); l > 0 && l <= 8 {
			s.MAC = net.HardwareAddr(r[1060 : 1060+l]).String()
		}
		if p, ok := ifPrev[luid]; ok && el > 0.2 && s.In >= p[0] && s.Out >= p[1] {
			s.InRate = float64(s.In-p[0]) / el
			s.OutRate = float64(s.Out-p[1]) / el
		}
		next[luid] = [2]uint64{s.In, s.Out}
		out = append(out, s)
	}
	ifPrev, ifAt, ifLast = next, now, out
	return summarizeIfaces(out)
}

func summarizeIfaces(l []ifaceStat) map[string]any {
	var in, out float64
	var tin, tout uint64
	for _, s := range l {
		in += s.InRate
		out += s.OutRate
		tin += s.In
		tout += s.Out
	}
	return map[string]any{"down": in, "up": out, "totalIn": tin, "totalOut": tout, "ifaces": l}
}

// ---------- conexiones por proceso ----------

type conn struct {
	Proto  string `json:"proto"`
	Local  string `json:"local"`
	Remote string `json:"remote"`
	State  string `json:"state"`
}

type netProc struct {
	PID     uint32  `json:"pid"`
	Name    string  `json:"name"`
	Exe     string  `json:"exe"`
	Path    string  `json:"path"`
	Est     int     `json:"est"`
	Listen  int     `json:"listen"`
	UDP     int     `json:"udp"`
	In      uint64  `json:"in"`
	Out     uint64  `json:"out"`
	InRate  float64 `json:"inRate"`
	OutRate float64 `json:"outRate"`
	Conns   []conn  `json:"conns"`
}

var tcpStates = map[uint32]string{1: "Cerrada", 2: "Escuchando", 3: "SYN enviado", 4: "SYN recibido", 5: "Establecida", 6: "FIN espera 1", 7: "FIN espera 2", 8: "Cerrando", 9: "Último ACK", 10: "Cerrando", 11: "Esperando cierre", 12: "Eliminando"}

func netPort(p uint32) uint16 { return uint16(p&0xff)<<8 | uint16(p>>8&0xff) }

func ip4(v uint32) string {
	return net.IPv4(byte(v), byte(v>>8), byte(v>>16), byte(v>>24)).String()
}

func extTable(proc *syscall.LazyProc, af, class uint32) []byte {
	var size uint32
	proc.Call(0, uintptr(unsafe.Pointer(&size)), 1, uintptr(af), uintptr(class), 0)
	for tries := 0; tries < 4 && size > 0; tries++ {
		buf := make([]byte, size+4096)
		size = uint32(len(buf))
		r, _, _ := proc.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 1, uintptr(af), uintptr(class), 0)
		if r == 0 {
			return buf[:size]
		}
		if r != 122 { // ERROR_INSUFFICIENT_BUFFER
			return nil
		}
	}
	return nil
}

// estadísticas por conexión TCP (requiere administrador)
type estatsKey struct {
	v6  bool
	row [52]byte
}

type rodData struct {
	DataBytesOut      uint64
	DataSegsOut       uint64
	DataBytesIn       uint64
	DataSegsIn        uint64
	SegsOut           uint64
	SegsIn            uint64
	SoftErrors        uint32
	SoftErrorReason   uint32
	SndUna            uint32
	SndNxt            uint32
	SndMax            uint32
	ThruBytesAcked    uint64
	RcvNxt            uint32
	ThruBytesReceived uint64
}

var (
	netMu      sync.Mutex
	estEnabled = map[estatsKey]uint32{}    // conexión -> pid
	estLast    = map[estatsKey][2]uint64{} // último total leído
	pidClosed  = map[uint32][2]uint64{}    // tráfico de conexiones ya cerradas
	pidPrevTot = map[uint32][2]uint64{}
	netPrevAt  time.Time
)

func readEStats(k estatsKey, enable bool) (uint64, uint64, bool) {
	setP, getP := pSetPerTcpEStats, pGetPerTcpEStats
	if k.v6 {
		setP, getP = pSetPerTcp6EStats, pGetPerTcp6EStats
	}
	if getP.Find() != nil {
		return 0, 0, false
	}
	row := k.row
	if enable {
		rw := byte(1)
		if r, _, _ := setP.Call(uintptr(unsafe.Pointer(&row[0])), 1, uintptr(unsafe.Pointer(&rw)), 0, 1, 0); r != 0 {
			return 0, 0, false
		}
	}
	var rod rodData
	r, _, _ := getP.Call(uintptr(unsafe.Pointer(&row[0])), 1, 0, 0, 0, 0, 0, 0, uintptr(unsafe.Pointer(&rod)), 0, unsafe.Sizeof(rod))
	if r != 0 {
		return 0, 0, false
	}
	return rod.DataBytesIn, rod.DataBytesOut, true
}

func networkProcesses() map[string]any {
	netMu.Lock()
	defer netMu.Unlock()
	procs, _ := smp.get(900 * time.Millisecond)
	byPID := map[uint32]rawProc{}
	for _, p := range procs {
		byPID[p.PID] = p
	}
	m := map[uint32]*netProc{}
	get := func(pid uint32) *netProc {
		np := m[pid]
		if np == nil {
			p := byPID[pid]
			np = &netProc{PID: pid, Exe: p.Name, Path: p.Path}
			if pid == 0 {
				np.Exe = "System Idle"
			}
			if pid == 4 {
				np.Exe = "System"
			}
			vi := cachedVersion(p.Path)
			np.Name = vi.Desc
			if np.Name == "" {
				np.Name = strings.TrimSuffix(np.Exe, filepath.Ext(np.Exe))
			}
			m[pid] = np
		}
		return np
	}
	seen := map[estatsKey]bool{}
	admin := adminCached

	// TCP IPv4: MIB_TCPROW_OWNER_PID (24 bytes)
	if b := extTable(pGetExtendedTcpTable, 2, 5); len(b) >= 4 {
		n := int(binary.LittleEndian.Uint32(b))
		for i := 0; i < n && 4+(i+1)*24 <= len(b); i++ {
			r := b[4+i*24:]
			st := binary.LittleEndian.Uint32(r)
			la, lp := binary.LittleEndian.Uint32(r[4:]), binary.LittleEndian.Uint32(r[8:])
			ra, rp := binary.LittleEndian.Uint32(r[12:]), binary.LittleEndian.Uint32(r[16:])
			pid := binary.LittleEndian.Uint32(r[20:])
			np := get(pid)
			c := conn{Proto: "TCP", Local: ip4(la) + ":" + itoa(int64(netPort(lp))), State: tcpStates[st]}
			if st == 2 {
				np.Listen++
			} else {
				c.Remote = ip4(ra) + ":" + itoa(int64(netPort(rp)))
				if st == 5 {
					np.Est++
					if admin {
						var k estatsKey
						copy(k.row[:20], r[:20]) // MIB_TCPROW
						seen[k] = true
						_, en := estEnabled[k]
						if in, out, ok := readEStats(k, !en); ok {
							estEnabled[k] = pid
							estLast[k] = [2]uint64{in, out}
						}
					}
				}
			}
			if len(np.Conns) < 60 {
				np.Conns = append(np.Conns, c)
			}
		}
	}
	// TCP IPv6: MIB_TCP6ROW_OWNER_PID (56 bytes)
	if b := extTable(pGetExtendedTcpTable, 23, 5); len(b) >= 4 {
		n := int(binary.LittleEndian.Uint32(b))
		for i := 0; i < n && 4+(i+1)*56 <= len(b); i++ {
			r := b[4+i*56:]
			la := net.IP(append([]byte(nil), r[0:16]...))
			lp := binary.LittleEndian.Uint32(r[20:])
			ra := net.IP(append([]byte(nil), r[24:40]...))
			rp := binary.LittleEndian.Uint32(r[44:])
			st := binary.LittleEndian.Uint32(r[48:])
			pid := binary.LittleEndian.Uint32(r[52:])
			np := get(pid)
			c := conn{Proto: "TCPv6", Local: "[" + la.String() + "]:" + itoa(int64(netPort(lp))), State: tcpStates[st]}
			if st == 2 {
				np.Listen++
			} else {
				c.Remote = "[" + ra.String() + "]:" + itoa(int64(netPort(rp)))
				if st == 5 {
					np.Est++
					if admin {
						var k estatsKey
						k.v6 = true
						// MIB_TCP6ROW: State, LocalAddr[16], LocalScopeId, LocalPort, RemoteAddr[16], RemoteScopeId, RemotePort
						binary.LittleEndian.PutUint32(k.row[0:], st)
						copy(k.row[4:28], r[0:24])
						copy(k.row[28:52], r[24:48])
						seen[k] = true
						_, en := estEnabled[k]
						if in, out, ok := readEStats(k, !en); ok {
							estEnabled[k] = pid
							estLast[k] = [2]uint64{in, out}
						}
					}
				}
			}
			if len(np.Conns) < 60 {
				np.Conns = append(np.Conns, c)
			}
		}
	}
	// UDP (solo recuento de puertos abiertos)
	if b := extTable(pGetExtendedUdpTable, 2, 1); len(b) >= 4 {
		n := int(binary.LittleEndian.Uint32(b))
		for i := 0; i < n && 4+(i+1)*12 <= len(b); i++ {
			get(binary.LittleEndian.Uint32(b[4+i*12+8:])).UDP++
		}
	}
	if b := extTable(pGetExtendedUdpTable, 23, 1); len(b) >= 4 {
		n := int(binary.LittleEndian.Uint32(b))
		for i := 0; i < n && 4+(i+1)*28 <= len(b); i++ {
			get(binary.LittleEndian.Uint32(b[4+i*28+24:])).UDP++
		}
	}

	// conexiones cerradas: su tráfico se acumula al proceso
	for k, pid := range estEnabled {
		if !seen[k] {
			l := estLast[k]
			c := pidClosed[pid]
			pidClosed[pid] = [2]uint64{c[0] + l[0], c[1] + l[1]}
			delete(estEnabled, k)
			delete(estLast, k)
		}
	}
	now := time.Now()
	el := now.Sub(netPrevAt).Seconds()
	totals := map[uint32][2]uint64{}
	for k, pid := range estEnabled {
		l := estLast[k]
		t := totals[pid]
		totals[pid] = [2]uint64{t[0] + l[0], t[1] + l[1]}
	}
	for pid, c := range pidClosed {
		t := totals[pid]
		totals[pid] = [2]uint64{t[0] + c[0], t[1] + c[1]}
		if _, alive := byPID[pid]; !alive {
			delete(pidClosed, pid)
		}
	}
	var list []netProc
	for pid, np := range m {
		if np.Est == 0 && np.Listen == 0 && np.UDP == 0 {
			continue
		}
		t := totals[pid]
		np.In, np.Out = t[0], t[1]
		if p, ok := pidPrevTot[pid]; ok && el > 0.2 {
			if t[0] >= p[0] {
				np.InRate = float64(t[0]-p[0]) / el
			}
			if t[1] >= p[1] {
				np.OutRate = float64(t[1]-p[1]) / el
			}
		}
		list = append(list, *np)
	}
	pidPrevTot = totals
	netPrevAt = now
	sort.Slice(list, func(i, j int) bool {
		a, b := list[i], list[j]
		if a.InRate+a.OutRate != b.InRate+b.OutRate {
			return a.InRate+a.OutRate > b.InRate+b.OutRate
		}
		if a.In+a.Out != b.In+b.Out {
			return a.In+a.Out > b.In+b.Out
		}
		return a.Est > b.Est
	})
	return map[string]any{"procs": list, "totals": netTotals(), "admin": admin}
}
