//go:build windows

package main

// Acceso al Registro de Windows (solo biblioteca estándar).

import (
	"encoding/binary"
	"errors"
	"syscall"
	"unsafe"
)

const (
	HKCR = uintptr(0x80000000)
	HKCU = uintptr(0x80000001)
	HKLM = uintptr(0x80000002)

	KEY_READ        = 0x20019
	KEY_WRITE       = 0x20006
	KEY_ALL         = 0xF003F
	KEY_WOW64_64KEY = 0x0100
	KEY_WOW64_32KEY = 0x0200

	REG_SZ        = 1
	REG_EXPAND_SZ = 2
	REG_BINARY    = 3
	REG_DWORD     = 4
	REG_MULTI_SZ  = 7
	REG_QWORD     = 11
)

var (
	pRegOpenKeyExW    = advapi32.NewProc("RegOpenKeyExW")
	pRegCloseKey      = advapi32.NewProc("RegCloseKey")
	pRegEnumKeyExW    = advapi32.NewProc("RegEnumKeyExW")
	pRegEnumValueW    = advapi32.NewProc("RegEnumValueW")
	pRegQueryValueExW = advapi32.NewProc("RegQueryValueExW")
	pRegSetValueExW   = advapi32.NewProc("RegSetValueExW")
	pRegDeleteValueW  = advapi32.NewProc("RegDeleteValueW")
	pRegDeleteTreeW   = advapi32.NewProc("RegDeleteTreeW")
	pRegCreateKeyExW  = advapi32.NewProc("RegCreateKeyExW")

	pExpandEnvironmentStringsW = kernel32.NewProc("ExpandEnvironmentStringsW")
)

type regKey uintptr

func regOpen(root uintptr, path string, access uint32) (regKey, error) {
	var h uintptr
	r, _, _ := pRegOpenKeyExW.Call(root, uintptr(unsafe.Pointer(u16(path))), 0, uintptr(access), uintptr(unsafe.Pointer(&h)))
	if r != 0 {
		return 0, syscall.Errno(r)
	}
	return regKey(h), nil
}

func regCreate(root uintptr, path string, access uint32) (regKey, error) {
	var h uintptr
	r, _, _ := pRegCreateKeyExW.Call(root, uintptr(unsafe.Pointer(u16(path))), 0, 0, 0, uintptr(access), 0, uintptr(unsafe.Pointer(&h)), 0)
	if r != 0 {
		return 0, syscall.Errno(r)
	}
	return regKey(h), nil
}

func (k regKey) Close() { pRegCloseKey.Call(uintptr(k)) }

func (k regKey) Subkeys() []string {
	var out []string
	buf := make([]uint16, 512)
	for i := 0; ; i++ {
		n := uint32(len(buf))
		r, _, _ := pRegEnumKeyExW.Call(uintptr(k), uintptr(i), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)), 0, 0, 0, 0)
		if r != 0 {
			break
		}
		out = append(out, syscall.UTF16ToString(buf[:n]))
	}
	return out
}

type regValue struct {
	Name string
	Type uint32
	Data []byte
}

func (k regKey) Values() []regValue {
	var out []regValue
	name := make([]uint16, 16384)
	for i := 0; ; i++ {
		nl := uint32(len(name))
		var typ, dl uint32
		r, _, _ := pRegEnumValueW.Call(uintptr(k), uintptr(i), uintptr(unsafe.Pointer(&name[0])), uintptr(unsafe.Pointer(&nl)), 0, uintptr(unsafe.Pointer(&typ)), 0, uintptr(unsafe.Pointer(&dl)))
		if r != 0 {
			break
		}
		nm := syscall.UTF16ToString(name[:nl])
		data, t, err := k.raw(nm)
		if err != nil {
			continue
		}
		out = append(out, regValue{nm, t, data})
	}
	return out
}

func (k regKey) raw(name string) ([]byte, uint32, error) {
	var typ, n uint32
	np := u16(name)
	r, _, _ := pRegQueryValueExW.Call(uintptr(k), uintptr(unsafe.Pointer(np)), 0, uintptr(unsafe.Pointer(&typ)), 0, uintptr(unsafe.Pointer(&n)))
	if r != 0 {
		return nil, 0, syscall.Errno(r)
	}
	if n == 0 {
		return []byte{}, typ, nil
	}
	if n > 4<<20 {
		return nil, 0, errors.New("valor demasiado grande")
	}
	buf := make([]byte, n)
	r, _, _ = pRegQueryValueExW.Call(uintptr(k), uintptr(unsafe.Pointer(np)), 0, uintptr(unsafe.Pointer(&typ)), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)))
	if r != 0 {
		return nil, 0, syscall.Errno(r)
	}
	return buf[:n], typ, nil
}

func utf16Bytes(b []byte) string {
	if len(b) < 2 {
		return ""
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(b[i*2:])
	}
	return syscall.UTF16ToString(u)
}

func (v regValue) String() string {
	switch v.Type {
	case REG_SZ:
		return utf16Bytes(v.Data)
	case REG_EXPAND_SZ:
		return expandEnv(utf16Bytes(v.Data))
	case REG_DWORD:
		if len(v.Data) >= 4 {
			return itoa(int64(binary.LittleEndian.Uint32(v.Data)))
		}
	case REG_QWORD:
		if len(v.Data) >= 8 {
			return itoa(int64(binary.LittleEndian.Uint64(v.Data)))
		}
	}
	return ""
}

// Str devuelve un valor de texto (expandiendo variables de entorno).
func (k regKey) Str(name string) string {
	d, t, err := k.raw(name)
	if err != nil {
		return ""
	}
	return regValue{name, t, d}.String()
}

func (k regKey) Uint(name string) (uint64, bool) {
	d, t, err := k.raw(name)
	if err != nil {
		return 0, false
	}
	switch {
	case t == REG_DWORD && len(d) >= 4:
		return uint64(binary.LittleEndian.Uint32(d)), true
	case (t == REG_QWORD || t == REG_BINARY) && len(d) >= 8:
		return binary.LittleEndian.Uint64(d), true
	case t == REG_BINARY && len(d) >= 4:
		return uint64(binary.LittleEndian.Uint32(d)), true
	}
	return 0, false
}

func (k regKey) SetBinary(name string, data []byte) error {
	var p uintptr
	if len(data) > 0 {
		p = uintptr(unsafe.Pointer(&data[0]))
	}
	r, _, _ := pRegSetValueExW.Call(uintptr(k), uintptr(unsafe.Pointer(u16(name))), 0, REG_BINARY, p, uintptr(len(data)))
	if r != 0 {
		return syscall.Errno(r)
	}
	return nil
}

func (k regKey) SetDword(name string, v uint32) error {
	r, _, _ := pRegSetValueExW.Call(uintptr(k), uintptr(unsafe.Pointer(u16(name))), 0, REG_DWORD, uintptr(unsafe.Pointer(&v)), 4)
	if r != 0 {
		return syscall.Errno(r)
	}
	return nil
}

func (k regKey) DeleteValue(name string) error {
	r, _, _ := pRegDeleteValueW.Call(uintptr(k), uintptr(unsafe.Pointer(u16(name))))
	if r != 0 {
		return syscall.Errno(r)
	}
	return nil
}

func regDeleteTree(root uintptr, path string, view uint32) error {
	// abre el padre y borra el árbol del hijo
	parent, child := path, ""
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '\\' {
			parent, child = path[:i], path[i+1:]
			break
		}
	}
	if child == "" {
		return errors.New("ruta de registro no válida")
	}
	k, err := regOpen(root, parent, KEY_ALL|view)
	if err != nil {
		return err
	}
	defer k.Close()
	r, _, _ := pRegDeleteTreeW.Call(uintptr(k), uintptr(unsafe.Pointer(u16(child))))
	if r != 0 {
		return syscall.Errno(r)
	}
	// RegDeleteTree deja la clave vacía en algunas versiones: se elimina también
	pDel := advapi32.NewProc("RegDeleteKeyExW")
	pDel.Call(uintptr(k), uintptr(unsafe.Pointer(u16(child))), uintptr(view), 0)
	return nil
}

func regKeyExists(root uintptr, path string, view uint32) bool {
	k, err := regOpen(root, path, KEY_READ|view)
	if err != nil {
		return false
	}
	k.Close()
	return true
}

func expandEnv(s string) string {
	if s == "" {
		return s
	}
	buf := make([]uint16, 2048)
	n, _, _ := pExpandEnvironmentStringsW.Call(uintptr(unsafe.Pointer(u16(s))), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 || int(n) > len(buf) {
		return s
	}
	return syscall.UTF16ToString(buf[:n])
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b [24]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func rootName(root uintptr) string {
	switch root {
	case HKCU:
		return "HKEY_CURRENT_USER"
	case HKLM:
		return "HKEY_LOCAL_MACHINE"
	case HKCR:
		return "HKEY_CLASSES_ROOT"
	}
	return "?"
}
