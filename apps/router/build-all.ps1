$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $scriptDir

$OUT = "dist"
if (!(Test-Path $OUT)) {
    New-Item -ItemType Directory -Force -Path $OUT | Out-Null
}

$VERSION = if ($env:VERSION) { $env:VERSION } else { "1.1.1" }
$LDFLAGS = "-s -w -X main.Version=$VERSION"

Write-Host "Building sirin-client v$VERSION for 5 architectures..." -ForegroundColor Cyan

# 1. MIPS (big-endian)
Write-Host "Compiling mips..."
$env:CGO_ENABLED = "0"; $env:GOOS = "linux"; $env:GOARCH = "mips"; $env:GOMIPS = "softfloat"
go build -ldflags="$LDFLAGS" -o "$OUT/sirin-client-mips" .

# 2. MIPSLE (little-endian)
Write-Host "Compiling mipsle..."
$env:CGO_ENABLED = "0"; $env:GOOS = "linux"; $env:GOARCH = "mipsle"; $env:GOMIPS = "softfloat"
go build -ldflags="$LDFLAGS" -o "$OUT/sirin-client-mipsle" .

# 3. ARMv7
Write-Host "Compiling armv7..."
$env:CGO_ENABLED = "0"; $env:GOOS = "linux"; $env:GOARCH = "arm"; $env:GOARM = "7"
go build -ldflags="$LDFLAGS" -o "$OUT/sirin-client-armv7" .

# 4. ARM64
Write-Host "Compiling arm64..."
$env:CGO_ENABLED = "0"; $env:GOOS = "linux"; $env:GOARCH = "arm64"
go build -ldflags="$LDFLAGS" -o "$OUT/sirin-client-arm64" .

# 5. x86_64
Write-Host "Compiling x86_64..."
$env:CGO_ENABLED = "0"; $env:GOOS = "linux"; $env:GOARCH = "amd64"
go build -ldflags="$LDFLAGS" -o "$OUT/sirin-client-x86_64" .

Write-Host "`nSizes:" -ForegroundColor Green
Get-ChildItem -Path "$OUT/sirin-client-*" -Exclude "*.tar.gz" | Select-Object Name, Length

# Packaging into .tar.gz
Write-Host "`nPackaging into tar.gz..." -ForegroundColor Cyan
$archs = @("mips", "mipsle", "armv7", "arm64", "x86_64")
foreach ($arch in $archs) {
    $tarFile = "$OUT/sirin-client-$VERSION-$arch.tar.gz"
    tar.exe -czf $tarFile -C $OUT "sirin-client-$arch" -C $scriptDir "init.d/sirin" -C "$scriptDir/configs" "client.toml.example" -C $scriptDir "configs/client.toml.example" -C $scriptDir "luci-app-sirin"
}

Write-Host "`nArchives:" -ForegroundColor Green
Get-ChildItem -Path "$OUT/*.tar.gz" | Select-Object Name, Length
