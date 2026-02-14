#Requires -Version 5.1
$ErrorActionPreference = "Stop"

$Repo = "708u/cctidy"
$Binary = "cctidy"

function Get-CctidyVersion {
    if ($env:VERSION) {
        return ($env:VERSION -replace '^v', '')
    }

    $release = Invoke-RestMethod `
        -Uri "https://api.github.com/repos/$Repo/releases/latest" `
        -Headers @{ "User-Agent" = "cctidy-installer" }
    $version = $release.tag_name -replace '^v', ''

    if (-not $version) {
        throw "Failed to detect latest version"
    }

    return $version
}

function Get-CctidyArch {
    switch ($env:PROCESSOR_ARCHITECTURE) {
        "AMD64" { return "x86_64" }
        "ARM64" { return "arm64" }
        "x86"   { return "i386" }
        default {
            throw "Unsupported architecture: $_"
        }
    }
}

function Install-Cctidy {
    $version = Get-CctidyVersion
    $arch = Get-CctidyArch
    $installDir = if ($env:INSTALL_DIR) {
        $env:INSTALL_DIR
    }
    else {
        Join-Path $env:USERPROFILE ".local\bin"
    }

    $archive = "${Binary}_Windows_${arch}.zip"
    $checksums = "${Binary}_${version}_checksums.txt"
    $baseUrl = "https://github.com/$Repo/releases/download/v$version"

    Write-Host "Installing $Binary v$version (Windows/$arch)..."

    $tmpDir = Join-Path ([IO.Path]::GetTempPath()) `
        ([Guid]::NewGuid().ToString())
    New-Item -ItemType Directory -Path $tmpDir |
        Out-Null

    try {
        $archivePath = Join-Path $tmpDir $archive
        $checksumsPath = Join-Path $tmpDir $checksums

        Invoke-WebRequest -Uri "$baseUrl/$archive" `
            -OutFile $archivePath -UseBasicParsing
        Invoke-WebRequest -Uri "$baseUrl/$checksums" `
            -OutFile $checksumsPath -UseBasicParsing

        # Verify checksum
        $line = Get-Content $checksumsPath |
            Where-Object {
                $_ -match [regex]::Escape($archive)
            }

        if (-not $line) {
            throw "Checksum not found for $archive"
        }

        $expected = ($line -split '\s+')[0]
        $actual = (Get-FileHash -Path $archivePath `
            -Algorithm SHA256).Hash.ToLower()

        if ($expected -ne $actual) {
            throw @"
Checksum mismatch
  expected: $expected
  actual:   $actual
"@
        }

        # Extract and install
        $extractDir = Join-Path $tmpDir "extract"
        Expand-Archive -Path $archivePath `
            -DestinationPath $extractDir

        if (-not (Test-Path $installDir)) {
            New-Item -ItemType Directory `
                -Path $installDir | Out-Null
        }

        $binFile = Get-ChildItem -Path $extractDir `
            -Filter "$Binary.exe" -Recurse |
            Select-Object -First 1

        if (-not $binFile) {
            throw "Binary not found in archive"
        }

        $dest = Join-Path $installDir "$Binary.exe"
        Copy-Item -Path $binFile.FullName `
            -Destination $dest -Force

        Write-Host "Successfully installed $Binary to $dest"

        # Check PATH
        $userPath = [Environment]::GetEnvironmentVariable(
            "PATH", "User")
        if ($userPath -notlike "*$installDir*") {
            Write-Host ""
            Write-Host "Add $installDir to your PATH:"
            Write-Host (
                '  $p = [Environment]' +
                '::GetEnvironmentVariable("PATH", "User")'
            )
            Write-Host (
                '  [Environment]' +
                '::SetEnvironmentVariable(' +
                '"PATH", "' + $installDir +
                ';$p", "User")'
            )
        }
    }
    finally {
        Remove-Item -Recurse -Force $tmpDir `
            -ErrorAction SilentlyContinue
    }
}

Install-Cctidy
