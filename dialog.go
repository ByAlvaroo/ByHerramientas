//go:build windows

package main

// Selector nativo de carpetas/archivos de Windows (IFileOpenDialog, el moderno).

import (
	"errors"
	"runtime"
	"syscall"
	"unsafe"
)

type guid struct {
	D1 uint32
	D2 uint16
	D3 uint16
	D4 [8]byte
}

var (
	clsidFileOpenDialog = guid{0xDC1C5A9C, 0xE88A, 0x4DDE, [8]byte{0xA5, 0xA1, 0x60, 0xF8, 0x2A, 0x20, 0xAE, 0xF7}}
	iidIFileOpenDialog  = guid{0xD57C7288, 0xD4AD, 0x4768, [8]byte{0xBE, 0x02, 0x9D, 0x96, 0x95, 0x32, 0xD9, 0x60}}
)

func comCall(obj uintptr, idx int, args ...uintptr) uintptr {
	vtbl := *(*uintptr)(unsafe.Pointer(obj))
	fn := *(*uintptr)(unsafe.Pointer(vtbl + uintptr(idx)*unsafe.Sizeof(uintptr(0))))
	r, _, _ := syscall.SyscallN(fn, append([]uintptr{obj}, args...)...)
	return r
}

// pickPath abre el diálogo. folder=true para elegir carpeta. Devuelve "" si se cancela.
func pickPath(folder bool, title string) (string, error) {
	type res struct {
		p   string
		err error
	}
	ch := make(chan res, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		pCoInitializeEx.Call(0, 0x2|0x4) // APARTMENTTHREADED | DISABLE_OLE1DDE
		defer pCoUninitialize.Call()
		var dlg uintptr
		hr, _, _ := pCoCreateInstance.Call(uintptr(unsafe.Pointer(&clsidFileOpenDialog)), 0, 1, uintptr(unsafe.Pointer(&iidIFileOpenDialog)), uintptr(unsafe.Pointer(&dlg)))
		if hr != 0 || dlg == 0 {
			ch <- res{"", errors.New("no se pudo abrir el selector")}
			return
		}
		defer comCall(dlg, 2) // Release
		var opts uint32
		comCall(dlg, 10, uintptr(unsafe.Pointer(&opts))) // GetOptions
		opts |= 0x40 | 0x800                             // FORCEFILESYSTEM | PATHMUSTEXIST
		if folder {
			opts |= 0x20 // PICKFOLDERS
		} else {
			opts |= 0x1000 // FILEMUSTEXIST
		}
		comCall(dlg, 9, uintptr(opts))                        // SetOptions
		comCall(dlg, 17, uintptr(unsafe.Pointer(u16(title)))) // SetTitle
		owner, _, _ := pGetForegroundWindow.Call()
		if hr := comCall(dlg, 3, owner); hr != 0 { // Show
			ch <- res{"", nil} // cancelado
			return
		}
		var item uintptr
		if comCall(dlg, 20, uintptr(unsafe.Pointer(&item))) != 0 || item == 0 { // GetResult
			ch <- res{"", errors.New("no se pudo leer la selección")}
			return
		}
		defer comCall(item, 2)
		var name *uint16
		if comCall(item, 5, 0x80058000, uintptr(unsafe.Pointer(&name))) != 0 || name == nil { // GetDisplayName(SIGDN_FILESYSPATH)
			ch <- res{"", errors.New("la selección no es una ruta del sistema de archivos")}
			return
		}
		p := syscall.UTF16ToString(unsafe.Slice(name, 32768))
		pCoTaskMemFree.Call(uintptr(unsafe.Pointer(name)))
		ch <- res{p, nil}
	}()
	r := <-ch
	return r.p, r.err
}
