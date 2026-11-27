#!/usr/bin/env bash
# Windows 배포용 zip 을 만든다.
#
#   scripts/build-windows.sh            # 64비트(amd64) → zip
#   scripts/build-windows.sh amd64 1.2.0   # 버전을 직접 지정 (기본은 wails.json 의 info.productVersion)
#   scripts/build-windows.sh arm64      # Windows on ARM
#
# 결과: build/bin/sobi-<버전>-windows-<아키텍처>.zip  (맨 위에 sobi-master.exe 한 개)
#   앱은 이 zip 을 내려받아 안의 exe 로 자기 자신을 바꿔치기한다(winupdate.go) — 파일 이름·위치를 바꾸지 말 것.
#   맥에서도 된다(Wails 가 Windows 용으로 크로스 컴파일). Windows 의 Git Bash 에서도 된다.
#   -clean 을 쓰지 않는다: 같은 릴리스에서 먼저 만든 DMG(build/bin)를 지우지 않기 위해서다.
set -euo pipefail

ARCH="${1:-amd64}"
case "$ARCH" in
amd64 | arm64) ;;
*)
	echo "사용법: $0 [amd64|arm64] [버전]" >&2
	exit 1
	;;
esac

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

VERSION="${2:-$(node -e 'console.log(JSON.parse(require("fs").readFileSync("wails.json", "utf8")).info.productVersion)')}"
if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
	echo "버전을 읽지 못했습니다: '$VERSION'" >&2
	exit 1
fi

if ! command -v wails >/dev/null; then
	echo "wails CLI 가 필요합니다: go install github.com/wailsapp/wails/v2/cmd/wails@latest" >&2
	exit 1
fi

echo "▶ wails build (windows/$ARCH)"
wails build -platform "windows/$ARCH" -o sobi-master.exe

EXE="build/bin/sobi-master.exe"
if [ ! -f "$EXE" ]; then
	echo "빌드 결과 $EXE 가 없습니다" >&2
	exit 1
fi

ZIP_NAME="sobi-${VERSION}-windows-${ARCH}.zip"
ZIP="build/bin/$ZIP_NAME"
rm -f "$ZIP"

echo "▶ zip"
cd build/bin
if command -v zip >/dev/null; then
	zip -q "$ZIP_NAME" sobi-master.exe
else
	powershell.exe -NoProfile -Command "Compress-Archive -Path 'sobi-master.exe' -DestinationPath '$ZIP_NAME' -Force"
fi
cd "$ROOT"

echo "준비 끝: $ZIP"
