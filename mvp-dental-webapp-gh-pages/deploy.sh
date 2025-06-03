#!/bin/bash

set -e

# Сборка проекта (на всякий случай)
npm run build

# Инициализация временной рабочей директории для gh-pages
git worktree add /tmp/gh-pages gh-pages || git checkout --orphan gh-pages

# Очистка старого содержимого
rm -rf /tmp/gh-pages/*

# Копирование новых файлов
cp -r dist/* /tmp/gh-pages/

cd /tmp/gh-pages

git add .
git commit -m "deploy: обновление сайта $(date '+%Y-%m-%d %H:%M:%S')" || echo "No changes to commit"
git push origin gh-pages

cd -
git worktree remove /tmp/gh-pages

echo "Деплой на GitHub Pages завершён!" 