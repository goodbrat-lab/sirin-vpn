#!/bin/sh
# scripts/feedback_digest.sh
# Собирает открытые Issue за последние 7 дней и группирует по платформе.
# Не требует jq — использует только gh CLI и awk.

set -e

# Проверка наличия gh CLI
if ! command -v gh >/dev/null 2>&1; then
  echo "Ошибка: утилита gh (GitHub CLI) не найдена в PATH." >&2
  echo "Установите GitHub CLI: https://cli.github.com/ или winget install --id GitHub.cli" >&2
  exit 1
fi

# Определение даты отсечки (7 дней назад)
if date -d "7 days ago" +%Y-%m-%d >/dev/null 2>&1; then
  cutoff=$(date -d "7 days ago" +%Y-%m-%d)
elif date -v-7d +%Y-%m-%d >/dev/null 2>&1; then
  cutoff=$(date -v-7d +%Y-%m-%d)
else
  cutoff=$(date +%Y-%m-%d)
fi

today=$(date +%Y-%m-%d)

echo "# Еженедельный дайджест фидбэка (Weekly Digest)"
echo "Период: с $cutoff по $today"
echo ""

gh issue list --state open --limit 200 --json number,title,labels,createdAt | awk -v cutoff="$cutoff" '
BEGIN {
  count = 0
}
{
  raw = raw $0 " "
}
END {
  print "| Платформа | Issue | Заголовок | Серьезность | Создан |"
  print "|---|---|---|---|---|"

  n = split(raw, objects, /\{"createdAt"/)
  for (i = 2; i <= n; i++) {
    item = "{\"createdAt\"" objects[i]

    cdate = ""
    if (match(item, /"createdAt":"([0-9]{4}-[0-9]{2}-[0-9]{2})/)) {
      m = substr(item, RSTART, RLENGTH)
      sub(/"createdAt":"/, "", m)
      cdate = m
    }

    if (cdate != "" && cdate < cutoff) {
      continue
    }

    num = "?"
    if (match(item, /"number":([0-9]+)/)) {
      m = substr(item, RSTART, RLENGTH)
      sub(/"number":/, "", m)
      num = m
    }

    title = ""
    if (match(item, /"title":"([^"]+)"/)) {
      m = substr(item, RSTART, RLENGTH)
      sub(/"title":"/, "", m)
      sub(/"$/, "", m)
      title = m
    }

    platform = "Не указана"
    if (item ~ /platform\/android/) {
      platform = "Android"
    } else if (item ~ /platform\/windows/) {
      platform = "Windows"
    } else if (item ~ /platform\/router/) {
      platform = "Router"
    }

    severity = "minor"
    if (item ~ /severity\/blocker/) {
      severity = "**blocker**"
    } else if (item ~ /severity\/major/) {
      severity = "*major*"
    } else if (item ~ /severity\/minor/) {
      severity = "minor"
    }

    if (num != "?") {
      printf "| %s | #%s | %s | %s | %s |\n", platform, num, title, severity, cdate
      count++
    }
  }

  if (count == 0) {
    print "| *(нет открытых задач)* | — | — | — | — |"
  }

  print ""
  printf "Всего открытых задач за 7 дней: %d\n", count
}
'
