# 소비마스터 (sobi-master)

[English](README.md) | **한국어**

가족 가계부 데스크톱 앱 (Go + Wails v2 + React/TS + Supabase/Postgres).

카드사/은행 CSV 내역을 가져오거나 수동으로 등록하고, 한 번 분류해 두면
"가맹점명 + 금액 구간" 핑거프린트 규칙으로 다음 달부터 자동 분류된다.
예: LG유플러스 자동결제 3건(금액만 다름)을 각각 아빠/아이/인터넷으로
한 번 라벨링하면 이후 매달 자동으로 누구 요금인지 구분된다.

## 기능

- **대시보드** — 월별 수입/지출/이체 요약과 전월 대비 증감, 알림 패널(예산 초과·카드 실적
  임박·미분류 누적), 6개월 추이 차트, 일별 지출 + 누적선, 카테고리별 예산 게이지,
  카테고리/구성원 도넛 차트, 지출 TOP 가맹점, 카드 실적 위젯
- **거래내역** — 수동 등록(현금·회비·경조사), 가맹점/메모 검색과 금액/기간/카테고리/귀속자/
  결제수단 필터 및 정렬, 모달 수정, 다중 선택 일괄 분류/삭제(되돌리기), "규칙으로 미분류 분류"
  소급 적용, 현재 화면 CSV 내보내기(엑셀 호환 UTF-8 BOM)
- **카드** — 카드사·결제일·실적기간·실적한도 등록, 현재 실적기간 달성 여부 추적,
  카드별 가맹점/카테고리 지출 분석, 칩 색상 지정
- **통계**(전용 탭) — 카드/귀속자/카테고리별 일별·누적 지출(기준·모드 토글), 전월 동일시점
  누적 비교, 요일별 평균 지출, 카테고리별 6개월 추이, 카드별 카테고리 구성(적층 막대),
  귀속자별 분석(카테고리 구성·6개월 추이·구성원 요약: 총액/비중/최다 카테고리·가맹점/전월 대비),
  카드 실적기간 누적 진행률, 고정비·정기결제 추적(이번 달 빠짐/예정), 연간 요약·전년 대비
- **예산** — 카테고리별 "매월 기본 예산" + "특정 월" 오버라이드, 대시보드 사용률 게이지와 초과 알림
- **가져오기** — 카드사/은행 CSV 내역, 컬럼 자동 인식(이용일/가맹점/금액/입금/출금),
  EUC-KR 인코딩 지원, 중복 건너뛰기
- **자동 분류** — 분류를 확정할 때마다 핑거프린트 규칙(가맹점 + 금액 ±8%)을 학습.
  수동 등록·가져오기 시 일치하는 규칙이 폼 기본값보다 우선해 자동 분류된다.
  설정 탭에서 규칙을 직접 추가/수정/삭제
- **다크 모드** — 헤더 토글, 다음 실행에도 유지

## 실행

```sh
wails dev      # 개발 모드 (핫 리로드)
wails build    # 배포 빌드 → build/bin/sobi.app (Windows 에서 빌드하면 .exe)
```

## 설정 파일 (config.json) — 필수

앱이 Supabase 에 접속하려면 설정 파일이 있어야 한다. 파일이 없으면
첫 실행 때 빈 템플릿이 자동 생성되므로 그 파일을 열어 채우면 된다.

### 경로

| OS      | 경로                                                  |
|---------|-------------------------------------------------------|
| macOS   | `~/Library/Application Support/sobi/config.json`       |
| Windows | `%AppData%\sobi\config.json` (보통 `C:\Users\<사용자명>\AppData\Roaming\sobi\config.json`) |
| Linux   | `~/.config/sobi/config.json`                            |

### 내용

```json
{
  "database_url": "postgresql://postgres.xxxxxxxxxxxx:비밀번호@aws-1-ap-southeast-1.pooler.supabase.com:5432/postgres"
}
```

- `database_url` 은 Supabase 대시보드 → 상단 **Connect** 버튼 → **Session pooler** URI 를 복사해서 넣는다.
  (Direct connection 은 IPv6 전용이라 일반 가정망에서는 Session pooler 권장.
  Transaction pooler(6543 포트)를 넣어도 앱이 자동으로 호환 모드로 동작한다.)
- `[YOUR-PASSWORD]` 부분을 실제 DB 비밀번호로 바꾼다.
  비밀번호에 `* : @ / !` 같은 특수문자가 있어도 그대로 붙여넣으면 된다 (앱이 자동 인코딩).
- Windows 메모장으로 저장해도 된다 (UTF-8 BOM 허용).

같은 `database_url` 을 다른 PC 의 config.json 에 넣으면 같은 가계부를 공유한다.

### 데이터 이전

앱이 접속했을 때 Supabase 가 비어 있고, 같은 폴더에 이전 버전의 로컬 DB
(`sobi.db`)가 있으면 모든 데이터를 자동으로 Supabase 로 옮긴다. (로컬 파일은 백업용으로 남는다)

### 로컬 백업

운영 데이터베이스는 Supabase 그대로지만, 앱이 정기적으로 로컬 SQLite 파일에도 스냅샷을
백업한다 — 시작 직후·실행 중 6시간마다·종료 시. 최근 7개를 보관하고 오래된 것은 지운다.
**설정 → 로컬 백업**에서 "지금 백업"으로 수동 실행도 가능하다. 파일은
`<설정 디렉토리>/backups/sobi_YYYYMMDD_HHMMSS.db` (Windows 는 `%AppData%\sobi\backups\`)에
쌓이며, 그 자체로 열람·복원 가능한 온전한 DB 다.

## 로그 파일 (sobi.log)

DB 연결 실패, 등록/수정 오류 등 모든 백엔드 오류가 기록된다.
문제가 생기면 이 파일을 먼저 확인한다. (직접 만들 필요 없음 — 앱이 자동 생성)

| OS      | 경로                                              |
|---------|----------------------------------------------------|
| macOS   | `~/Library/Application Support/sobi/sobi.log`       |
| Windows | `%AppData%\sobi\sobi.log`                            |
| Linux   | `~/.config/sobi/sobi.log`                            |

연결에 실패해도 앱은 종료되지 않고 화면 상단에 오류 배너가 뜨며,
설정을 고친 뒤 "다시 연결" 버튼을 누르면 재시작 없이 복구된다.

## 구조

- `app.go` — Wails 바인딩 (프론트 API, 연결 재시도/로깅, 대시보드 통합 엔드포인트,
  CSV 내보내기, 알림/정기결제/예산 헬퍼)
- `internal/store` — Supabase(Postgres) 스키마/쿼리, 집계·통계(`analytics.go`, `stats.go`),
  예산(`budget.go`), 설정/로그 경로, SQLite 마이그레이션
- `internal/classifier` — 자동 분류 규칙 학습/매칭 (금액 허용오차 ±8%)
- `internal/importer` — 카드사/은행 CSV 파서 (헤더 키워드 자동 인식, EUC-KR 지원)
- `frontend/src/pages` — 대시보드 / 거래내역(수동 등록 포함) / 카드 / 통계 / 가져오기 / 설정

## 테스트

DB 테스트는 Postgres 가 필요하다 (없으면 자동 건너뜀):

```sh
docker run -d --rm --name sobi-test-pg -e POSTGRES_PASSWORD=test -e POSTGRES_DB=sobi -p 55432:5432 postgres:16-alpine
TEST_DATABASE_URL='postgres://postgres:test@localhost:55432/sobi?sslmode=disable' go test ./...
docker stop sobi-test-pg
```

## 개념

- **귀속자(member)**: 돈이 누구를 위해 쓰였는가 (아빠/엄마/아이/공동)
- **카테고리(category)**: 용도. kind 가 수입/지출/이체를 구분 (급여·통신비·대출상환·회비·투자이체 등)
- **결제수단(payment method)**: 어느 카드/현금/계좌로 나갔는가. 카드는 카드 탭에서 결제일/실적기간/실적한도 관리
- **예산(budget)**: 카테고리별 월 지출 한도(선택). 설정 탭에서 매월 기본을 정하고 특정 월만 따로 지정할 수 있으며, 대시보드가 사용률·초과를 추적
- **규칙(rule)**: 분류를 확정할 때마다 자동 학습. 일치하면 수동 등록 기본값보다 우선한다. 설정 탭에서 확인/추가/수정/삭제하며, 여러 달에 걸쳐 반복되면 통계 탭 고정비 추적에 활용된다
