#!/usr/bin/env bash
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROFILE="${1:-$ROOT/core/coverage.out}"
MIN="${CORE_COVERAGE_MIN:-80}"

if [ ! -f "$PROFILE" ]; then
    printf 'coverage-gate: нет файла профиля %s\n' "$PROFILE" >&2
    exit 1
fi

python3 - "$PROFILE" "$MIN" <<'PY'
import sys
from pathlib import Path

profile_path = Path(sys.argv[1])
minimum = int(sys.argv[2])

total = covered = 0
skipped = []

for line in profile_path.read_text().splitlines()[1:]:
    name, statements, count = line.rsplit(' ', 2)
    name = name.split(':')[0]
    if '/cmd/' in name or name.endswith('/migrations/embed.go'):
        skipped.append(name)
        continue
    statements = int(statements)
    total += statements
    if int(count) > 0:
        covered += statements

percent = covered * 100 // total if total else 0

print(f'coverage-gate: прикладной код {covered}/{total} = {percent}% (порог {minimum}%)')
for name in sorted(set(skipped)):
    print(f'coverage-gate: исключено {name}')

if percent < minimum:
    print(f'coverage-gate: НЕ ПРОЙДЕНО — нужно минимум {minimum}%', file=sys.stderr)
    sys.exit(1)

print('coverage-gate: пройдено')
PY
