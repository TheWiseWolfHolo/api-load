# 在 Windows 本机运行与 CI 相同的 Go 测试集合。
# 跳过 .github/fork-skipped-tests.txt 与 .github/windows-local-skipped-tests.txt 中的测试，其余参数原样传给 go test。
# 例：pwsh scripts/go-test-windows.ps1
#     pwsh scripts/go-test-windows.ps1 -run TestOpenRouter ./internal/execution/...
param([Parameter(ValueFromRemainingArguments = $true)][string[]]$GoTestArgs)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot

$names = foreach ($list in '.github/fork-skipped-tests.txt', '.github/windows-local-skipped-tests.txt') {
    Get-Content -LiteralPath (Join-Path $root $list) |
        ForEach-Object { $_.Trim() } |
        Where-Object { $_ -and -not $_.StartsWith('#') }
}
$skip = '^(' + ($names -join '|') + ')$'

# 部分测试会执行 .github/scripts 下的 Bash 脚本。PATH 里的 system32\bash.exe 是 WSL 入口，
# 没有安装发行版时会失败，所以优先使用 Git for Windows 自带的 bash。
$git = Get-Command git -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
if ($git) {
    $gitBashDir = Join-Path (Split-Path -Parent (Split-Path -Parent $git.Source)) 'usr\bin'
    if (Test-Path -LiteralPath (Join-Path $gitBashDir 'bash.exe')) {
        $env:PATH = $gitBashDir + [IO.Path]::PathSeparator + $env:PATH
    }
}

$packages = @('.', './internal/...', './tools/migrate-api-load-v1/...')
$explicitPackages = $GoTestArgs | Where-Object { $_ -like './*' -or $_ -eq '.' }
if ($explicitPackages) { $packages = @() }

Push-Location -LiteralPath $root
try {
    & go test -count=1 -timeout=20m -skip $skip @GoTestArgs @packages
    exit $LASTEXITCODE
} finally {
    Pop-Location
}
