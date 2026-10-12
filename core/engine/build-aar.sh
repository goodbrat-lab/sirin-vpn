#!/bin/sh
# build-aar.sh — сборка Go-движка в AAR для Android
#
# Требования:
#   - Go 1.27+
#   - Android SDK (переменная ANDROID_HOME или стандартные пути)
#   - Android NDK (переменная ANDROID_NDK_HOME или $ANDROID_HOME/ndk/*)
#   - Java 17+ (переменная JAVA_HOME или стандартные пути)
#
# Использование:
#   ./build-aar.sh
#
# Сборка engine.aar.
#
# Скрипт существует ради ОДНОГО флага: -ldflags=-checklinkname=0.
#
# Без него сборка падает так:
#
#   link: github.com/wlynxg/anet: invalid reference to net.zoneCache
#
# Причина: pion/turn безусловно импортирует transport/v4/stdnet
# (turn/client.go:17), тот тянет github.com/wlynxg/anet, а anet через
# //go:linkname лезет во внутренний net.zoneCache
# (anet/interface_android.go:164). Go с версии 1.23 такие ссылки
# запрещает. Обойти выбором версии нельзя — stdnet импортируется
# безусловно и в turn v3, и в v4.
#
# Ошибка ни словом не упоминает TURN, так что забытый флаг стоит часа
# недоумения. Отсюда и скрипт.
#
# ПРОВЕРЕНО 26.08.2026, Go 1.27.0: флаг ВСЁ ЕЩЁ НУЖЕН.
#
# Проверял не рассуждением, а сборкой. `go build ./...` тут ничего не
# доказывает: библиотеку он не компонует, и ошибка появляется только на
# настоящей линковке в libgojni.so.
#
# Повторить проверку:
#
#   gomobile bind -target=android/arm64 -androidapi 26 -o /tmp/probe.aar .
#
# Собралось — флаг ниже можно убирать.
#
# Обходных путей на сегодня нет:
#
#   anet уже САМОЙ СВЕЖЕЙ версии v0.0.5, и запрещённая ссылка в ней;
#   pion/transport/v4 v4.1.0 (новее нашей) тянет anet ровно так же —
#     проверял, зависимость не отвалилась;
#   отказаться от pion/turn нельзя: это релейная ступень целиком.
#
# Что снимет флаг: выход anet новее v0.0.5 без //go:linkname. Проверять
# при каждом обновлении зависимостей — командой выше, она стоит минуту.
#
# Цена флага, чтобы не забывалась: он снимает проверку компоновщика
# ГЛОБАЛЬНО, на всю сборку, а не только для anet. Сузить нельзя, это
# ключ линкера.
#
# Перед запуском владелец выгружает античит:  fltmc unload FACEIT
#
# Android SDK — берётся из окружения, если не задано — типичные пути
if [ -z "$ANDROID_HOME" ]; then
    if [ -d "$HOME/AppData/Local/Android/Sdk" ]; then
        ANDROID_HOME="$HOME/AppData/Local/Android/Sdk"       # Windows
    elif [ -d "$HOME/Library/Android/sdk" ]; then
        ANDROID_HOME="$HOME/Library/Android/sdk"             # macOS
    elif [ -d "$HOME/Android/Sdk" ]; then
        ANDROID_HOME="$HOME/Android/Sdk"                     # Linux
    else
        echo "ERROR: ANDROID_HOME не задан и не найден в типичных путях"
        echo "Установи ANDROID_HOME вручную перед запуском скрипта"
        exit 1
    fi
fi

# NDK — аналогично, но с проверкой версии
if [ -z "$ANDROID_NDK_HOME" ]; then
    # Ищем любой установленный NDK в $ANDROID_HOME/ndk/
    NDK_DIR=$(ls -d "$ANDROID_HOME/ndk/"*/ 2>/dev/null | sort -V | tail -1)
    if [ -n "$NDK_DIR" ]; then
        ANDROID_NDK_HOME="${NDK_DIR%/}"
    else
        echo "WARN: NDK не найден в $ANDROID_HOME/ndk/"
        echo "Установи NDK через Android SDK Manager"
        exit 1
    fi
fi

# JAVA_HOME — если не задан, ищем JBR из Android Studio
if [ -z "$JAVA_HOME" ]; then
    if [ -d "/c/Program Files/Android/Android Studio/jbr" ]; then
        JAVA_HOME="/c/Program Files/Android/Android Studio/jbr"
    elif [ -d "C:/Program Files/Android/Android Studio/jbr" ]; then
        JAVA_HOME="C:/Program Files/Android/Android Studio/jbr"
    elif [ -d "/Applications/Android Studio.app/Contents/jbr/Contents/Home" ]; then
        JAVA_HOME="/Applications/Android Studio.app/Contents/jbr/Contents/Home"
    elif command -v java >/dev/null 2>&1; then
        # Если java уже в PATH, берем ее родительский каталог
        JAVA_HOME="$(dirname "$(dirname "$(command -v java)")")"
    else
        echo "ERROR: JAVA_HOME не задан и не найден в типичных путях"
        echo "Установи JAVA_HOME вручную перед запуском скрипта"
        exit 1
    fi
fi

export ANDROID_HOME
export ANDROID_NDK_HOME
export JAVA_HOME
export PATH="$JAVA_HOME/bin:$PATH"

set -e

cd "$(dirname "$0")"

# АРХИТЕКТУРЫ ВЫБИРАЮТСЯ КЛЮЧОМ.
#
#   build-aar.sh          — только arm64-v8a. Умолчание для отладки:
#                           собирается втрое быстрее и APK втрое легче,
#                           а телефон владельца всё равно arm64;
#   build-aar.sh --all    — все три. Обязательно для релиза и Play.
#
# Имена у gomobile свои и с именами Android не совпадают:
#   android/arm64 -> arm64-v8a
#   android/arm   -> armeabi-v7a
#   android/amd64 -> x86_64
#
# 32-битный x86 не собираем: устройств на нём практически не осталось.
# Список обязан совпадать с abiFilters в app/build.gradle.kts — иначе в
# бандл попадёт архитектура без движка.
#
# Флаг -checklinkname=0 нужен всем одинаково: он про линковку, а не про
# архитектуру, и запрещённая ссылка anet на net.zoneCache существует в
# любой сборке под Android.
#
# ЗАБЫТЬ --all ПЕРЕД РЕЛИЗОМ НЕЛЬЗЯ: сборка релиза проверяет AAR и
# падает с внятным текстом, если архитектур в нём меньше трёх. Проверка
# в app/build.gradle.kts.

TARGET="android/arm64"
WHAT="только arm64-v8a (отладка)"
if [ "$1" = "--all" ]; then
	TARGET="android/arm64,android/arm,android/amd64"
	WHAT="три архитектуры (релиз)"
fi

go vet ./...
mkdir -p ../../apps/android/app/libs
gomobile bind \
	-target="$TARGET" \
	-androidapi 26 \
	-ldflags=-checklinkname=0 \
	-o ../../apps/android/app/libs/engine.aar \
	.

echo "engine.aar собран: $WHAT"
echo "  Целевая папка: $(pwd)/../../apps/android/app/libs"
