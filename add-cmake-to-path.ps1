# Add CMake to PATH
$cmakePath = "C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\Common7\IDE\CommonExtensions\Microsoft\CMake\CMake\bin"

$currentPath = [Environment]::GetEnvironmentVariable("Path", "User")

if (-not $currentPath.Contains($cmakePath)) {
    [Environment]::SetEnvironmentVariable("Path", $currentPath + ";" + $cmakePath, "User")
    Write-Host "✓ CMake added to user PATH"
    Write-Host ""
    Write-Host "IMPORTANT: Close and reopen VS Code (or PowerShell) for PATH to take effect"
    Write-Host ""
    Write-Host "After reopening, verify with: cmake --version"
} else {
    Write-Host "✓ CMake already in PATH"
}

Write-Host ""
Write-Host "Current session PATH (for this PowerShell only):"
$env:Path = $env:Path + ";" + $cmakePath
Write-Host "CMake is now available in THIS terminal session"
Write-Host ""
Write-Host "Testing: cmake --version"
& "$cmakePath\cmake.exe" --version
