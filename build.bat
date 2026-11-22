@echo off
rem Compila Vantix Utilities (requiere Go 1.22 o superior: https://go.dev/dl/)
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
set /p VER=<version.txt
go build -trimpath -ldflags "-s -w -H windowsgui -X main.appVersion=%VER%" -o Vantix.exe .
if %errorlevel%==0 (echo Listo: Vantix.exe) else (echo Error al compilar)
pause
