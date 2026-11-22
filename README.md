# Vantix Utilities

Utilidades del sistema para Windows 10/11 en un único ejecutable portable.

## ⬇️ Descargar

**[Descargar Vantix.exe (última versión)](https://github.com/ByAlvaroo/Vantix/releases/latest/download/Vantix.exe)**

No necesita instalación. Ábrelo y listo. Cuando se publica una versión nueva, la propia aplicación avisa y se actualiza sola.

## Herramientas

**Rendimiento**
- **Resumen**: CPU, memoria, procesos, tiempo encendido y discos.
- **Procesos**: lista clasificada (innecesario, aplicación, segundo plano, controlador, seguridad, sistema) y limpieza recomendada.
- **Hardware**: uso por núcleo, frecuencia, GPU NVIDIA (temperatura, VRAM, consumo, ventilador), RAM con módulos, discos con temperatura y todos los sensores de LibreHardwareMonitor.
- **Red**: velocidad en tiempo real, programas conectados y tráfico por programa.
- **Modo juego**: cierra lo innecesario, plan de energía de máximo rendimiento y libera memoria en espera.

**Limpieza**
- **Limpiador**: temporales, Windows Update, cachés de navegadores y aplicaciones, informes de errores, Papelera.
- **Espacio en disco**: mapa de carpetas, archivos más grandes y espacio por tipo.
- **Duplicados**: copias idénticas verificadas con SHA-256.
- **Programas de inicio**: activar/desactivar lo que arranca con Windows.
- **Desinstalar programas**: desinstalador oficial + limpieza de restos.

**Archivos**
- **Eliminación forzada**: libera y elimina carpetas bloqueadas por procesos.
- **Búsqueda de archivos** en todas las unidades.
- **Renombrar en masa** con vista previa y deshacer.

**Sistema**
- **Información del sistema** exportable (TXT / informe HTML).
- **Guía de uso** integrada y **Ajustes**.

## Publicar una versión nueva

1. Cambia el número en `version.txt` (por ejemplo `1.2.0`).
2. Haz *commit* y *push* (GitHub Desktop).
3. GitHub Actions compila `Vantix.exe` y crea la *release* `v1.2.0` automáticamente.
4. Las copias instaladas detectan la versión nueva y se actualizan desde la aplicación.

## Compilar a mano

Requiere Go 1.22+. En Windows ejecuta `build.bat`; desde Linux/macOS:

```
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -H windowsgui" -o Vantix.exe .
```

El icono, el manifiesto y la información de versión están en `rsrc_windows_amd64.syso` (se regenera con `python tools/mksyso.py`).

HECHO POR @BYALVARO__
