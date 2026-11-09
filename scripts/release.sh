#!/usr/bin/env bash
# 새 버전 배포 준비: 버전 올리기 → DMG 빌드 → 변경 사항 초안.
#
#   scripts/release.sh 1.1.0            # 유니버설 DMG
#   scripts/release.sh 1.1.0 arm64      # Apple Silicon 전용 DMG
#
# 하는 일
#   1. wails.json 의 info.productVersion 을 새 버전으로 바꾼다 (앱 화면에 보이는 버전)
#   2. scripts/build-dmg.sh 로 DMG 를 만든다
#   3. 지난 태그 이후 커밋 제목으로 변경 사항 초안을 만든다 (feat → 새 기능, fix → 고친 점, perf → 빨라진 점)
#
# 하지 않는 일 (끝에 안내한다)
#   커밋·태그·푸시와 GitHub 릴리스 만들기. 공개 저장소에 올리는 일이라 직접 확인하고 한다.
#   앱은 GitHub Releases 의 v<버전> 릴리스를 보고 '새 버전 받기'를 띄우고, 릴리스 설명을 '바뀌는 점'으로,
#   첨부한 DMG 를 내려받을 파일로 쓴다.
set -euo pipefail

VERSION="${1:-}"
ARCH="${2:-universal}"
if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
	echo "사용법: $0 <버전 예: 1.1.0> [universal|arm64|amd64]" >&2
	exit 1
fi
TAG="v$VERSION"

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if git rev-parse -q --verify "refs/tags/$TAG" >/dev/null; then
	echo "$TAG 태그가 이미 있습니다" >&2
	exit 1
fi

echo "▶ 버전 $VERSION (wails.json)"
node -e '
const fs = require("fs");
const j = JSON.parse(fs.readFileSync("wails.json", "utf8"));
j.info = j.info || {};
j.info.productVersion = process.argv[1];
fs.writeFileSync("wails.json", JSON.stringify(j, null, 2) + "\n");
' "$VERSION"

scripts/build-dmg.sh "$ARCH"
DMG="build/bin/sobi-${VERSION}-macOS-${ARCH}.dmg"

echo "▶ 변경 사항 초안"
PREV="$(git describe --tags --abbrev=0 --match 'v[0-9]*' 2>/dev/null || true)"
RANGE="HEAD"
[ -n "$PREV" ] && RANGE="$PREV..HEAD"
NOTES="build/bin/release-notes-$TAG.md"
section() { # $1=제목 $2=커밋 접두어
	local lines
	lines="$(git log --no-merges --pretty=format:%s "$RANGE" | sed -nE "s/^$2(\([^)]*\))?!?: *(.+)$/- \2/p")"
	if [ -n "$lines" ]; then
		printf '## %s\n%s\n\n' "$1" "$lines"
	fi
}
{
	section "새 기능" "feat"
	section "고친 점" "fix"
	section "빨라진 점" "perf"
} >"$NOTES"
[ -s "$NOTES" ] || echo "- (변경 사항을 적어 주세요)" >"$NOTES"

cat <<EOF

✔ 준비 끝: $DMG
  변경 사항 초안: $NOTES  (${PREV:-처음}부터 지금까지의 커밋 — 다듬어서 쓰세요)

남은 일
  1) 버전 변경을 커밋하고 태그를 올립니다
       git commit -am "release: $TAG" && git tag $TAG && git push origin HEAD $TAG
  2) GitHub 에서 릴리스를 만듭니다
       https://github.com/suslmk-lee/sobi-master/releases/new?tag=$TAG
       - 제목: $TAG
       - 설명: 위 초안을 붙여 넣고 다듬기 (앱의 '바뀌는 점'에 그대로 보입니다)
       - 파일: $DMG 를 끌어다 첨부
     '릴리스 게시'를 누르면, 이전 버전 앱에서 '새 버전 받기'가 나타납니다.
EOF
