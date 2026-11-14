#!/usr/bin/env bash
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT" || exit 1

line() { printf '\n\033[1m%s\033[0m\n' "$1"; }
note() { printf '  %s\n' "$1"; }

line '1. Слои'
for dir in core web ai mocks specs docs infra scripts; do
    if [ -d "$dir" ]; then
        files=$(find "$dir" -type f -not -path '*/node_modules/*' | wc -l)
        lines=$(find "$dir" -type f \( -name '*.go' -o -name '*.ts' -o -name '*.tsx' -o -name '*.py' -o -name '*.css' -o -name '*.sql' -o -name '*.yml' \) -not -path '*/node_modules/*' -exec cat {} + 2>/dev/null | wc -l)
        note "$(printf '%-8s %3d файлов  ~%6d строк' "$dir" "$files" "$lines")"
    else
        note "$(printf '%-8s ОТСУТСТВУЕТ' "$dir")"
    fi
done

line '2. Backend: эндпоинты'
if [ -d core ]; then
    routes=$(grep -rhoE '(Get|Post|Patch|Put|Delete)\("[^"]*"' core/internal --include='*.go' 2>/dev/null | sort -u | wc -l)
    note "уникальных роутов: $routes"
fi

line '3. Backend: тесты и покрытие'
if [ -d core ]; then
    note "файлов *_test.go: $(find core -name '*_test.go' | wc -l)"
    note "функций Test*:     $(grep -rh '^func Test' core --include='*_test.go' | wc -l)"
    note "пустых модулей:    $(for d in core/internal/*/; do [ -z "$(find "$d" -name '*.go' ! -name 'doc.go' -print -quit)" ] && basename "$d"; done | tr '\n' ' ')"
fi

line '4. Frontend: роуты и фичи'
if [ -d web/src ]; then
    note "роутов:  $(find web/src/routes -name '*.tsx' | wc -l)"
    note "фич:    $(find web/src/features -mindepth 1 -maxdepth 1 -type d 2>/dev/null | xargs -n1 basename 2>/dev/null | tr '\n' ' ')"
    note "компонентов ui: $(ls web/src/components/ui 2>/dev/null | tr '\n' ' ')"
    note "тестов web:     $(find web -name '*.test.*' -o -name '*.spec.*' 2>/dev/null | grep -v node_modules | wc -l)"
fi

line '5. AI-сервис'
if [ -d ai ]; then
    note "модулей python: $(find ai/app -name '*.py' | wc -l)"
    note "тестов python:  $(find ai -name 'test_*.py' | wc -l)"
    note "есть mcp.py:    $([ -f ai/app/mcp.py ] && echo да || echo 'нет — MCP-сервис не поднимется')"
    note "есть cron.py:   $([ -f ai/app/cron.py ] && echo да || echo 'нет')"
else
    note 'каталога ai/ нет'
fi

line '6. Обязательные артефакты сдачи'
check() {
    if [ -e "$1" ]; then note "OK      $1"; else note "НЕТ     $1"; fi
}
check 'specs/001-ai-buddy/spec.md'
check 'docs/demo-script.md'
check 'docs/api-contract.md'
check 'README.md'
check '.github/workflows/ci.yml'
check 'docker-compose.yml'
note "openapi-файлов:   $(find . -name 'openapi*.y*ml' -not -path '*/node_modules/*' | wc -l)"
note "презентаций:      $(find . \( -iname '*.pptx' \) -not -path '*/node_modules/*' | wc -l)"
note "видео:            $(find . \( -iname '*.mp4' -o -iname '*.webm' \) -not -path '*/node_modules/*' | wc -l)"

line '7. Docker: сервисы compose против каталогов'
if command -v docker >/dev/null 2>&1; then
    note "сервисы: $(grep -E '^  [a-z0-9-]+:' docker-compose.yml | tr -d ' :' | tr '\n' ' ')"
    for svc in ai mcp mocks cron; do
        if grep -q "context: ./$svc" docker-compose.yml 2>/dev/null; then
            if [ -d "$svc" ]; then note "OK      сервис $svc (каталог есть)"; else note "НЕТ     сервис $svc (каталога $svc/ нет — сборка упадёт)"; fi
        fi
    done
fi

line '8. Git: ветки и незалитое'
note "текущая ветка: $(git branch --show-current)"
base_ref="origin/main"
git rev-parse --verify "$base_ref" >/dev/null 2>&1 || base_ref="main"
note "сравниваем с: $base_ref"
for branch in $(git branch -r | grep -v HEAD | tr -d ' ' | grep -vE 'origin/main$'); do
    ahead=$(git rev-list --count "$base_ref".."$branch" 2>/dev/null)
    merged=$(git merge-base --is-ancestor "$branch" "$base_ref" 2>/dev/null && echo "в main" || echo "НЕ в main")
    note "$(printf '%-42s %s (+%s)' "$branch" "$merged" "$ahead")"
done
note "изменённых файлов: $(git status --short | grep -v '^??' | wc -l), новых: $(git status --short | grep '^??' | wc -l)"

line '9. План дня 1: статусы по владельцам'
if [ -f plan_day1.md ]; then
    python3 - <<'PY'
import re
text = open('plan_day1.md', encoding='utf-8').read()
for sec in re.split(r'\n## ', text):
    title = sec.split('\n')[0].strip()
    if not re.match(r'^\d\.', title):
        continue
    tasks = re.findall(r'\*\*([A-Z]-\d+)\*\*', sec)
    done = set(re.findall(r'^- \[[xX]\].*\*\*([A-Z]-\d+)\*\*', sec, re.M))
    if not tasks:
        continue
    ready = [t for t in tasks if t in done]
    late = [t for t in tasks if t not in done]
    print(f"  {title[:40]:42} {len(ready):2}/{len(tasks):2} готово   не начато: {' '.join(late) if late else '—'}")
PY
fi

line '10. Гейты дня 1'
python3 - <<'PY'
import re
try:
    text = open('plan_day1.md', encoding='utf-8').read()
except FileNotFoundError:
    raise SystemExit(0)
sec = re.search(r'## 6\. Gate дня 1(.*?)\n---', text, re.S)
if not sec:
    raise SystemExit(0)
marks = re.findall(r'^- \[([xX ])\] \*\*(G-\d+)\*\*', sec.group(1), re.M)
ready = [g for mark, g in marks if mark.lower() == 'x']
open_ = [g for mark, g in marks if mark.lower() != 'x']
print(f"  закрыто: {len(ready)}/{len(marks)}  ({' '.join(ready)})")
print(f"  открыто: {' '.join(open_)}")
PY

printf '\n\033[1mГотово.\033[0m\n'
