# Vantix Utilities

Utilidades del sistema para Windows 10/11 en un único ejecutable portable.

- **Resumen**: CPU, memoria, procesos, tiempo encendido y discos en tiempo real.
- **Procesos**: lista completa clasificada (innecesario, aplicación, segundo plano, controlador, seguridad, sistema) y limpieza recomendada.
- **Eliminación forzada**: detecta qué procesos bloquean una carpeta o archivo, los finaliza y lo elimina (o lo envía a la Papelera). Permite programar la eliminación al reiniciar.
- **Búsqueda de archivos**: búsqueda por nombre en todas las unidades (7 modos, filtros, exportación CSV).
- **Guía de uso** integrada y **Ajustes** (exclusiones, cerrar siempre, frecuencia de actualización).

## Uso
Abre `Vantix.exe`. No necesita instalación ni conexión a Internet. La interfaz se abre en una ventana propia de Microsoft Edge (modo aplicación, perfil independiente). Al cerrar la ventana, la aplicación se cierra.

## Compilar
Requiere Go 1.22+. En Windows ejecuta `build.bat`; desde Linux/macOS:

```
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -H windowsgui" -o Vantix.exe .
```

El icono, el manifiesto y la información de versión están en `rsrc_windows_amd64.syso` (se regenera con `python tools/mksyso.py` desde la carpeta `tools`).

## Estructura
- `main.go` – servidor local, ventana, configuración y API
- `procs.go` – procesos: muestreo, clasificación y finalización
- `unlock.go` – eliminación forzada (Administrador de reinicio de Windows)
- `search.go` – búsqueda de archivos en paralelo
- `dialog.go` – selector nativo de carpetas/archivos
- `win.go` – llamadas a la API de Windows
- `ui/` – interfaz (HTML, CSS y JS)

HECHO POR @BYALVARO__
