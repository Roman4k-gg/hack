param(
    [Parameter(Position = 0)]
    [ValidateSet('help', 'up', 'up-dev', 'up-demo', 'down', 'seed', 'demo-reset', 'logs', 'ps', 'test', 'test-core', 'coverage', 'lint', 'fmt', 'build')]
    [string]$Target = 'help',

    [switch]$Detached
)

$ErrorActionPreference = 'Stop'

function Compose {
    param([string[]]$Args)
    docker compose @Args
}

function ComposeProfile {
    param(
        [string[]]$Profiles,
        [string[]]$Args
    )
    $composeArgs = @()
    foreach ($profile in $Profiles) {
        $composeArgs += '--profile'
        $composeArgs += $profile
    }
    $composeArgs += $Args
    docker compose @composeArgs
}

function ShowHelp {
    Write-Host 'PowerShell-обёртка над Makefile (для Windows без GNU Make).'
    Write-Host ''
    Write-Host 'Использование:'
    Write-Host '  pwsh .\make.ps1 <target>'
    Write-Host ''
    Write-Host 'Доступные цели:'
    Write-Host '  up            поднять postgres, redis и core'
    Write-Host '  up-dev        поднять всё вместе с web и ai'
    Write-Host '  up-demo       поднять демо-стенд'
    Write-Host '  down          остановить сервисы'
    Write-Host '  seed          применить миграции и наполнить базу'
    Write-Host '  demo-reset    пересоздать базу и запустить seed'
    Write-Host '  logs          логи сервисов'
    Write-Host '  ps            список сервисов'
    Write-Host '  test          тесты всех слоёв'
    Write-Host '  test-core     тесты Go-сервиса'
    Write-Host '  coverage      покрытие Go-сервиса'
    Write-Host '  lint          линтеры'
    Write-Host '  fmt           форматирование'
    Write-Host '  build         сборка образов'
}

function TestCore {
    Push-Location (Join-Path $PSScriptRoot 'core')
    try {
        go test ./...
    }
    finally {
        Pop-Location
    }
}

function Coverage {
    Push-Location (Join-Path $PSScriptRoot 'core')
    try {
        go test ./... -race -coverprofile=coverage.out -covermode=atomic
        go tool cover -html=coverage.out -o coverage.html
        Write-Host 'coverage: core/coverage.html'
    }
    finally {
        Pop-Location
    }
}

function Lint {
    Push-Location (Join-Path $PSScriptRoot 'core')
    try {
        go vet ./...
        if (Get-Command golangci-lint -ErrorAction SilentlyContinue) {
            golangci-lint run ./...
        }
        else {
            Write-Host 'golangci-lint не найден, пропускаем'
        }
    }
    finally {
        Pop-Location
    }
}

function Fmt {
    Push-Location (Join-Path $PSScriptRoot 'core')
    try {
        gofmt -l -w .
    }
    finally {
        Pop-Location
    }
}

switch ($Target) {
    'help' { ShowHelp }
    'up' {
        Compose @('up', '--build', '-d', '--wait')
        Write-Host 'core:  http://localhost:8081/healthz'
        Write-Host 'ready: http://localhost:8081/readyz'
    }
    'up-dev' {
        ComposeProfile -Profiles @('dev') -Args @('up', '--build', '-d', '--wait')
        Write-Host 'web: http://localhost:3000'
    }
    'up-demo' {
        ComposeProfile -Profiles @('demo') -Args @('up', '--build', '-d', '--wait')
        Write-Host 'web:   http://localhost:3000'
        Write-Host 'mocks: http://localhost:8090/healthz'
    }
    'down' {
        ComposeProfile -Profiles @('dev', 'demo') -Args @('down', '--remove-orphans')
    }
    'seed' {
        ComposeProfile -Profiles @('demo') -Args @('run', '--rm', 'seed')
    }
    'demo-reset' {
        ComposeProfile -Profiles @('demo') -Args @('down', '-v')
        Compose @('up', '--build', '-d', '--wait')
        ComposeProfile -Profiles @('demo') -Args @('run', '--rm', 'seed')
    }
    'logs' { ComposeProfile -Profiles @('dev', 'demo') -Args @('logs', '-f', '--tail=100') }
    'ps' { ComposeProfile -Profiles @('dev', 'demo') -Args @('ps') }
    'test' {
        TestCore
        if (Test-Path (Join-Path $PSScriptRoot 'ai')) {
            Push-Location (Join-Path $PSScriptRoot 'ai')
            try { uv run pytest }
            finally { Pop-Location }
        }
        if (Test-Path (Join-Path $PSScriptRoot 'web')) {
            Push-Location (Join-Path $PSScriptRoot 'web')
            try { pnpm vitest run }
            finally { Pop-Location }
        }
    }
    'test-core' { TestCore }
    'coverage' { Coverage }
    'lint' { Lint }
    'fmt' { Fmt }
    'build' { Compose @('build') }
    default { ShowHelp; exit 1 }
}
