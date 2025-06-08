@echo off
echo ========================================
echo           Snoopr Client Builder
echo ========================================

if "%~1"=="" (
    echo Usage: buildclient.bat [SERVER_IP] [SERVER_PORT]
    echo Example: buildclient.bat 192.168.1.100 8080
    exit /b 1
)

if "%~2"=="" (
    echo Usage: buildclient.bat [SERVER_IP] [SERVER_PORT]
    echo Example: buildclient.bat 192.168.1.100 8080
    exit /b 1
)

set SERVER_IP=%1
set SERVER_PORT=%2

echo Building client for server: %SERVER_IP%:%SERVER_PORT%

:: Download dependencies
echo Installing Go dependencies...
go mod tidy

:: Create temporary client file with server details
echo Creating client configuration...
set CLIENT_FILE=cmd\client\main_build.go
copy cmd\client\main.go %CLIENT_FILE%

:: Replace placeholders with actual values
powershell -Command "(Get-Content %CLIENT_FILE%) -replace 'SERVER_IP_PLACEHOLDER', '%SERVER_IP%' | Set-Content %CLIENT_FILE%"
powershell -Command "(Get-Content %CLIENT_FILE%) -replace 'SERVER_PORT_PLACEHOLDER', '%SERVER_PORT%' | Set-Content %CLIENT_FILE%"

:: Build the client for Windows
echo Building Windows executable...
set GOOS=windows
set GOARCH=amd64
go build -ldflags="-H windowsgui -s -w" -o bin\snoopr-client.exe %CLIENT_FILE%

:: Cleanup
del %CLIENT_FILE%

if exist bin\snoopr-client.exe (
    echo.
    echo ========================================
    echo Build successful!
    echo Client executable: bin\snoopr-client.exe
    echo Server: %SERVER_IP%:%SERVER_PORT%
    echo ========================================
) else (
    echo.
    echo ========================================
    echo Build failed!
    echo ========================================
    exit /b 1
)

pause 