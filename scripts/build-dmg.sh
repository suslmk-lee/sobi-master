#!/usr/bin/env bash
# macOS 배포용 DMG 를 만든다.
#
#   scripts/build-dmg.sh            # 유니버설(Apple Silicon + Intel) → DMG
#   scripts/build-dmg.sh arm64      # Apple Silicon 전용 (빌드가 빠르다)
#   scripts/build-dmg.sh amd64      # Intel 전용
#
# 결과: build/bin/sobi-<버전>-macOS-<아키텍처>.dmg
#   DMG 를 열면 sobi.app, Applications 바로가기, '처음 실행 안내.txt' 가 보인다.
#   버전은 wails.json 의 info.productVersion 을 따른다.
#   wails build -clean 이 build/bin 을 비우므로 이전 DMG 는 지워진다.
#
# 서명:
#   기본은 ad-hoc 서명(인증서 없음). 받는 맥에서 처음 열 때 Gatekeeper 경고가 뜨며,
#   DMG 안의 안내대로 한 번 허용하면 된다.
#   Apple Developer ID 인증서가 있으면 환경변수로 정식 서명·공증까지 한다(경고 없이 열린다).
#     SIGN_IDENTITY="Developer ID Application: 이름 (TEAMID)"
#     NOTARY_PROFILE="sobi-notary"   # xcrun notarytool store-credentials 로 미리 저장한 프로필 이름
set -euo pipefail

ARCH="${1:-universal}"
case "$ARCH" in
universal | arm64 | amd64) ;;
*)
	echo "사용법: $0 [universal|arm64|amd64]" >&2
	exit 1
	;;
esac

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if ! command -v wails >/dev/null; then
	echo "wails CLI 가 필요합니다: go install github.com/wailsapp/wails/v2/cmd/wails@latest" >&2
	exit 1
fi

echo "▶ wails build (darwin/$ARCH)"
wails build -clean -platform "darwin/$ARCH"

APP="build/bin/sobi.app"
if [ ! -d "$APP" ]; then
	echo "빌드 결과 $APP 가 없습니다" >&2
	exit 1
fi
VERSION="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$APP/Contents/Info.plist")"
DMG="build/bin/sobi-${VERSION}-macOS-${ARCH}.dmg"

echo "▶ 서명"
if [ -n "${SIGN_IDENTITY:-}" ]; then
	codesign --force --deep --options runtime --timestamp --sign "$SIGN_IDENTITY" "$APP"
else
	codesign --force --deep --sign - "$APP"
fi
codesign --verify --deep --strict "$APP"

echo "▶ DMG 만들기"
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT
ditto "$APP" "$STAGE/sobi.app"
ln -s /Applications "$STAGE/Applications"
cp scripts/dmg-readme.txt "$STAGE/처음 실행 안내.txt"
rm -f "$DMG"
hdiutil create -volname "sobi ${VERSION}" -srcfolder "$STAGE" -fs HFS+ -format UDZO -ov "$DMG" >/dev/null

if [ -n "${SIGN_IDENTITY:-}" ]; then
	codesign --sign "$SIGN_IDENTITY" --timestamp "$DMG"
	if [ -n "${NOTARY_PROFILE:-}" ]; then
		echo "▶ 공증 (Apple 서버에서 몇 분 걸릴 수 있음)"
		xcrun notarytool submit "$DMG" --keychain-profile "$NOTARY_PROFILE" --wait
		xcrun stapler staple "$DMG"
	fi
fi

hdiutil verify "$DMG" >/dev/null
echo "✔ $DMG ($(du -h "$DMG" | cut -f1))"
