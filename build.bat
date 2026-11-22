@echo off
rem Compila Vantix Utilities (requiere Go 1.22 o superior: https://go.dev/dl/)
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
go build -trimpath -ldflags "-s -w -H windowsgui" -o Vantix.exe .
if %errorlevel%==0 (echo Listo: Vantix.exe) else (echo Error al compilar)
pause
