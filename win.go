//go:build windows

package main

// Declaraciones de la API de Windows usadas por la aplicación.
// Solo se usa la biblioteca estándar (syscall) para no depender de nada externo.

import (
	"strings"
	"syscall"
	"unsafe"
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	user32   = syscall.NewLazyDLL("user32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	ntdll    = syscall.NewLazyDLL("ntdll.dll")
	versionD = syscall.NewLazyDLL("version.dll")
	rstrtmgr = syscall.NewLazyDLL("rstrtmgr.dll")
	ole32    = syscall.NewLazyDLL("ole32.dll")
	dwmapi   = syscall.NewLazyDLL("dwmapi.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")

	pQueryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")
	pK32GetProcessMemoryInfo    = kernel32.NewProc("K32GetProcessMemoryInfo")
	pGetSystemTimes             = kernel32.NewProc("GetSystemTimes")
	pGlobalMemoryStatusEx       = kernel32.NewProc("GlobalMemoryStatusEx")
	pGetDiskFreeSpaceExW        = kernel32.NewProc("GetDiskFreeSpaceExW")
	pGetTickCount64             = kernel32.NewProc("GetTickCount64")
	pProcessIdToSessionId       = kernel32.NewProc("ProcessIdToSessionId")
	pGetLogicalDrives           = kernel32.NewProc("GetLogicalDrives")
	pGetDriveTypeW              = kernel32.NewProc("GetDriveTypeW")
	pGetVolumeInformationW      = kernel32.NewProc("GetVolumeInformationW")
	pFindFirstFileExW           = kernel32.NewProc("FindFirstFileExW")
	pFindNextFileW              = kernel32.NewProc("FindNextFileW")
	pMoveFileExW                = kernel32.NewProc("MoveFileExW")
	pSetFileAttributesW         = kernel32.NewProc("SetFileAttributesW")
	pReadProcessMemory          = kernel32.NewProc("ReadProcessMemory")
	pGetCurrentProcessId        = kernel32.NewProc("GetCurrentProcessId")
	pCreateMutexW               = kernel32.NewProc("CreateMutexW")

	pNtQueryInformationProcess = ntdll.NewProc("NtQueryInformationProcess")

	pEnumWindows              = user32.NewProc("EnumWindows")
	pGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	pIsWindowVisible          = user32.NewProc("IsWindowVisible")
	pGetWindow                = user32.NewProc("GetWindow")
	pGetWindowTextW           = user32.NewProc("GetWindowTextW")
	pGetWindowTextLengthW     = user32.NewProc("GetWindowTextLengthW")
	pGetWindowLongPtrW        = user32.NewProc("GetWindowLongPtrW")
	pGetForegroundWindow      = user32.NewProc("GetForegroundWindow")

	pShellExecuteW    = shell32.NewProc("ShellExecuteW")
	pSHFileOperationW = shell32.NewProc("SHFileOperationW")
	pIsUserAnAdmin    = shell32.NewProc("IsUserAnAdmin")

	pGetFileVersionInfoSizeW = versionD.NewProc("GetFileVersionInfoSizeW")
	pGetFileVersionInfoW     = versionD.NewProc("GetFileVersionInfoW")
	pVerQueryValueW          = versionD.NewProc("VerQueryValueW")

	pRmStartSession      = rstrtmgr.NewProc("RmStartSession")
	pRmRegisterResources = rstrtmgr.NewProc("RmRegisterResources")
	pRmGetList           = rstrtmgr.NewProc("RmGetList")
	pRmEndSession        = rstrtmgr.NewProc("RmEndSession")

	pCoInitializeEx   = ole32.NewProc("CoInitializeEx")
	pCoUninitialize   = ole32.NewProc("CoUninitialize")
	pCoCreateInstance = ole32.NewProc("CoCreateInstance")
	pCoTaskMemFree    = ole32.NewProc("CoTaskMemFree")

	pDwmGetWindowAttribute = dwmapi.NewProc("DwmGetWindowAttribute")
	_                      = advapi32
)

const (
	PROCESS_TERMINATE                 = 0x0001
	PROCESS_VM_READ                   = 0x0010
	PROCESS_QUERY_INFORMATION         = 0x0400
	PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
	SYNCHRONIZE                       = 0x00100000
	TH32CS_SNAPPROCESS                = 0x00000002
	FILE_ATTRIBUTE_NORMAL             = 0x80
	FILE_ATTRIBUTE_READONLY           = 0x1
	FILE_ATTRIBUTE_HIDDEN             = 0x2
	FILE_ATTRIBUTE_SYSTEM             = 0x4
	FILE_ATTRIBUTE_DIRECTORY          = 0x10
	FILE_ATTRIBUTE_REPARSE_POINT      = 0x400
	MOVEFILE_DELAY_UNTIL_REBOOT       = 0x4
)

type processMemoryCountersEx struct {
	Cb                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
	PrivateUsage               uintptr
}

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

// WIN32_FIND_DATAW con el tamaño correcto (la de syscall tiene un campo recortado).
type win32FindData struct {
	FileAttributes    uint32
	CreationTime      syscall.Filetime
	LastAccessTime    syscall.Filetime
	LastWriteTime     syscall.Filetime
	FileSizeHigh      uint32
	FileSizeLow       uint32
	Reserved0         uint32
	Reserved1         uint32
	FileName          [260]uint16
	AlternateFileName [14]uint16
}

func u16(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func ftU64(f syscall.Filetime) uint64 { return uint64(f.HighDateTime)<<32 | uint64(f.LowDateTime) }

// ftUnix convierte un FILETIME a segundos Unix.
func ftUnix(f syscall.Filetime) int64 {
	v := ftU64(f)
	if v == 0 {
		return 0
	}
	return int64(v/10000000) - 11644473600
}

// longPath añade el prefijo \\?\ para rutas largas en llamadas directas a la API.
func longPath(p string) string {
	if len(p) < 240 || strings.HasPrefix(p, `\\?\`) {
		return p
	}
	if strings.HasPrefix(p, `\\`) {
		return `\\?\UNC\` + p[2:]
	}
	return `\\?\` + p
}

func isAdmin() bool {
	r, _, _ := pIsUserAnAdmin.Call()
	return r != 0
}

func currentPID() uint32 {
	r, _, _ := pGetCurrentProcessId.Call()
	return uint32(r)
}

func shellExecute(verb, file, args string) error {
	var a *uint16
	if args != "" {
		a = u16(args)
	}
	r, _, e := pShellExecuteW.Call(0, uintptr(unsafe.Pointer(u16(verb))), uintptr(unsafe.Pointer(u16(file))), uintptr(unsafe.Pointer(a)), 0, 1)
	if r <= 32 {
		if r == 5 || r == 1223 {
			return syscall.Errno(1223)
		}
		return e
	}
	return nil
}

func setAttrs(p string, attrs uint32) {
	pSetFileAttributesW.Call(uintptr(unsafe.Pointer(u16(longPath(p)))), uintptr(attrs))
}

func scheduleDeleteOnReboot(p string) error {
	r, _, e := pMoveFileExW.Call(uintptr(unsafe.Pointer(u16(longPath(p)))), 0, MOVEFILE_DELAY_UNTIL_REBOOT)
	if r == 0 {
		return e
	}
	return nil
}

func memStatus() memoryStatusEx {
	var m memoryStatusEx
	m.Length = uint32(unsafe.Sizeof(m))
	pGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&m)))
	return m
}

func tick64() uint64 {
	r, _, _ := pGetTickCount64.Call()
	return uint64(r)
}

func systemTimes() (idle, kernel, user uint64, ok bool) {
	var i, k, u syscall.Filetime
	r, _, _ := pGetSystemTimes.Call(uintptr(unsafe.Pointer(&i)), uintptr(unsafe.Pointer(&k)), uintptr(unsafe.Pointer(&u)))
	return ftU64(i), ftU64(k), ftU64(u), r != 0
}

func diskSpace(root string) (free, total uint64, ok bool) {
	var fa, tot, tf uint64
	r, _, _ := pGetDiskFreeSpaceExW.Call(uintptr(unsafe.Pointer(u16(root))), uintptr(unsafe.Pointer(&fa)), uintptr(unsafe.Pointer(&tot)), uintptr(unsafe.Pointer(&tf)))
	return fa, tot, r != 0
}

type driveInfo struct {
	Path     string `json:"path"`
	Label    string `json:"label"`
	Kind     string `json:"kind"`
	Selected bool   `json:"selected"`
	Free     uint64 `json:"free"`
	Total    uint64 `json:"total"`
}

func listDrives() []driveInfo {
	mask, _, _ := pGetLogicalDrives.Call()
	var out []driveInfo
	for i := 0; i < 26; i++ {
		if mask&(1<<uint(i)) == 0 {
			continue
		}
		root := string(rune('A'+i)) + `:\`
		t, _, _ := pGetDriveTypeW.Call(uintptr(unsafe.Pointer(u16(root))))
		kind := ""
		switch t {
		case 2:
			kind = "removable"
		case 3:
			kind = "fixed"
		case 4:
			kind = "network"
		case 5:
			kind = "cdrom"
		case 6:
			kind = "ram"
		default:
			continue
		}
		d := driveInfo{Path: root, Kind: kind, Selected: kind == "fixed"}
		if kind != "cdrom" {
			var name [261]uint16
			r, _, _ := pGetVolumeInformationW.Call(uintptr(unsafe.Pointer(u16(root))), uintptr(unsafe.Pointer(&name[0])), 261, 0, 0, 0, 0, 0)
			if r != 0 {
				d.Label = syscall.UTF16ToString(name[:])
			}
			d.Free, d.Total, _ = diskSpace(root)
		}
		if d.Label == "" {
			switch kind {
			case "fixed":
				d.Label = "Disco local"
			case "removable":
				d.Label = "Extraíble"
			case "network":
				d.Label = "Unidad de red"
			case "cdrom":
				d.Label = "CD/DVD"
			default:
				d.Label = "Unidad"
			}
		}
		out = append(out, d)
	}
	return out
}

// ---------- Información de versión de ejecutables (descripción y editor) ----------

type verInfo struct {
	Desc    string
	Company string
	Product string
}

func fileVersionInfo(path string) verInfo {
	var vi verInfo
	p := u16(path)
	size, _, _ := pGetFileVersionInfoSizeW.Call(uintptr(unsafe.Pointer(p)), 0)
	if size == 0 || size > 4<<20 {
		return vi
	}
	buf := make([]byte, size)
	r, _, _ := pGetFileVersionInfoW.Call(uintptr(unsafe.Pointer(p)), 0, size, uintptr(unsafe.Pointer(&buf[0])))
	if r == 0 {
		return vi
	}
	var tp uintptr
	var tl uint32
	langs := []string{}
	r, _, _ = pVerQueryValueW.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(u16(`\VarFileInfo\Translation`))), uintptr(unsafe.Pointer(&tp)), uintptr(unsafe.Pointer(&tl)))
	if r != 0 && tl >= 4 {
		n := int(tl / 4)
		if n > 8 {
			n = 8
		}
		for i := 0; i < n; i++ {
			lang := *(*uint16)(unsafe.Pointer(tp + uintptr(i*4)))
			cp := *(*uint16)(unsafe.Pointer(tp + uintptr(i*4+2)))
			langs = append(langs, hex4(lang)+hex4(cp))
		}
	}
	langs = append(langs, "040904b0", "040904e4", "0c0a04b0", "000004b0")
	get := func(key string) string {
		for _, l := range langs {
			var vp uintptr
			var vl uint32
			r, _, _ := pVerQueryValueW.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(u16(`\StringFileInfo\`+l+`\`+key))), uintptr(unsafe.Pointer(&vp)), uintptr(unsafe.Pointer(&vl)))
			if r != 0 && vl > 0 && vp != 0 {
				s := unsafe.Slice((*uint16)(unsafe.Pointer(vp)), vl)
				if v := strings.TrimSpace(syscall.UTF16ToString(s)); v != "" {
					return v
				}
			}
		}
		return ""
	}
	vi.Desc = get("FileDescription")
	vi.Company = get("CompanyName")
	vi.Product = get("ProductName")
	return vi
}

func hex4(v uint16) string {
	const h = "0123456789abcdef"
	return string([]byte{h[v>>12&15], h[v>>8&15], h[v>>4&15], h[v&15]})
}

func unsafePtr(p *uint16) unsafe.Pointer { return unsafe.Pointer(p) }
