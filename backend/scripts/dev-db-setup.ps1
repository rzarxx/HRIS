# Membuat database pengembangan + role, lalu menulis backend/.env (tidak masuk Git).
# psql akan menanyakan kata sandi superuser "postgres"; kata sandi itu tidak disimpan.
$ErrorActionPreference = 'Stop'
$psql = 'C:\Program Files\PostgreSQL\18\bin\psql.exe'
$backend = Split-Path $PSScriptRoot -Parent
$envFile = Join-Path $backend '.env'

function New-Secret {
  $bytes = New-Object byte[] 24
  [Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
  -join ($bytes | ForEach-Object { $_.ToString('x2') })
}

function Read-EnvValue($name) {
  if (-not (Test-Path $envFile)) { return $null }
  foreach ($line in Get-Content $envFile) {
    if ($line -match "^$name=(.+)$") { return $Matches[1] }
  }
  return $null
}

function Get-PasswordFromDsn($dsn) {
  if ($dsn -match '://[^:]+:([^@]+)@') { return $Matches[1] }
  return $null
}

$migratePw = Get-PasswordFromDsn (Read-EnvValue 'MIGRATE_DATABASE_URL')
if (-not $migratePw) { $migratePw = New-Secret }
$appPw = Get-PasswordFromDsn (Read-EnvValue 'DATABASE_URL')
if (-not $appPw) { $appPw = New-Secret }
$roPw = Read-EnvValue 'HRIS_RO_PASSWORD'
if (-not $roPw) { $roPw = New-Secret }
$seedPw = Read-EnvValue 'SEED_SDM_PASSWORD'
if (-not $seedPw) { $seedPw = New-Secret }

$env:HRIS_MIGRATE_PW = $migratePw
$env:HRIS_APP_PW = $appPw
$env:HRIS_RO_PW = $roPw

& $psql -U postgres -h localhost -d postgres -f (Join-Path $PSScriptRoot 'dev-db-setup.sql')
if ($LASTEXITCODE -ne 0) { throw 'Penyiapan database gagal.' }

$content = @"
APP_ENV=development
HTTP_ADDR=127.0.0.1:8080
DATABASE_URL=postgres://hris_app:$appPw@localhost:5432/hris_dev?sslmode=disable
MIGRATE_DATABASE_URL=postgres://hris_migrate:$migratePw@localhost:5432/hris_dev?sslmode=disable
HRIS_RO_PASSWORD=$roPw
SEED_SDM_PASSWORD=$seedPw
JWT_SIGNING_KEY_PATH=.secrets/jwt_ed25519.pem
JWT_KEY_ID=dev-1
FIELD_ENCRYPTION_KEY_PATH=.secrets/field_encryption.key
MFA_REQUIRED=false
COOKIE_DOMAIN=
CORS_ALLOWED_ORIGINS=http://localhost:3000
LOG_LEVEL=info
DEV_SIMULATE_ATTENDANCE=true
"@
[IO.File]::WriteAllText($envFile, $content, (New-Object Text.UTF8Encoding($false)))
Write-Host "Selesai. Konfigurasi ditulis ke $envFile"
