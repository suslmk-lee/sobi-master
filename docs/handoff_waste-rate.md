# Handoff: 낭비율(지출 건강도) + 대시보드용 읽기 전용 뷰

- 작성일: 2026-11-27 (기준 커밋: main 874002b, v1.11.1)
- 요청 배경: `~/workspace/my-dashboard` 에 개인 대시보드(Rust + Tauri)를 새로 만든다. 거기서 "가계부 기록 기준 이번 달 낭비율" 과 월 요약·예산 현황을 보여 주려 한다. 지금 스키마와 로직에는 "낭비" 개념이 없다.
- 데이터 경로: 대시보드가 Supabase 에 **읽기 전용 역할 `dashboard_ro`** 로 직접 붙어 **`dash` 스키마의 뷰만** 읽는다. 계산식은 이 저장소가 관리하는 뷰 안에 둔다. 앱의 `GetSpendingHealth` 도 같은 뷰를 읽게 해서 앱과 대시보드 숫자가 항상 같게 한다.
  - 모바일 PWA(`sobi-master-mobile`)는 이 작업과 관계없다.
- 관련 문서: `~/workspace/my-dashboard/docs/handoff_dashboard-mvp.md` (대시보드 전체 계획)

---

## 0. 먼저 정할 것 (사용자 확인 필요)

아래 **기본 정의**로 진행하되, 사용자가 다르게 정하면 그쪽을 따른다.

> **낭비액** = ① `낭비` 로 분류한 카테고리 지출 + ② `선택` 으로 분류한 카테고리의 **예산 초과분**
> **낭비율** = 낭비액 ÷ 그달 총지출

- 총지출은 `MonthlySummary.TotalExpense` 와 같은 기준이다 (`direction='expense'`, `queries.go:821`). 앱 대시보드 숫자와 반드시 맞아야 한다.
- **예산 초과분은 이중으로 세지 않는다** (기본안, 사용자 확인 필요). 예산 사용액(`BudgetStatuses`)은 주 예산에 부 지출까지 합산하므로 그대로 쓰면 두 가지가 겹친다.
  - '선택' 주 아래 '낭비' 부의 지출이 낭비로 한 번, 주 예산 초과분으로 또 한 번 들어간다.
  - 주와 부에 모두 예산이 있으면 부의 초과가 주의 초과에도 들어간다. 낭비액이 총지출보다 커질 수도 있다.
  - 기본안: 거래 하나는 **가장 가까운 예산 하나**에만 묶는다(부에 예산이 있으면 부, 없으면 주). 예산마다 `초과분 = max(0, 그 예산에 묶인 '선택' 지출 − 예산액)`. 필수·낭비·미지정 지출은 초과분 계산에 넣지 않는다.
  - 그래서 `waste_amount ≤ total_expense` 가 항상 성립한다. 예산 화면의 사용액(주에 부 합산)과는 기준이 다르다는 점을 화면 설명에 적는다.
- 이체(`transfer`)와 수입은 빼고 계산한다.
- 보조 지표 두 가지를 함께 낸다:
  - **선택소비 비율** = 선택 카테고리 지출 ÷ 총지출
  - **미지정 비율** = 분류 안 된 카테고리 지출 ÷ 총지출. 이 값이 크면 낭비율을 믿기 어렵다는 표시다.
- 2단계(선택): 거래 한 건에 "낭비" 표시를 따로 다는 기능. 1단계에서는 하지 않는다.

## 1. 지금 있는 것 (재사용)

| 이미 있는 것 | 위치 | 쓰는 방법 |
|---|---|---|
| 고정비 표시 `categories.fixed_cost` + 상위 주 상속(`ParentFixed`) | `store.go:357`, `models.go:22-25`, `queries.go:51`, `queries.go:308 SetCategoryFixed`, `app.go:376` | **같은 패턴**으로 새 컬럼을 만든다 |
| 고정비 기본값을 한 번만 채우는 시드 (`seed.fixed_cost`) | `store.go:357-362` | 같은 방식으로 `seed.spend_class` |
| 지출 성격 분류식 `spendKind` (fixed / big / living) | `member_analysis.go:30-36` | 고정비는 `필수` 로 본다. `spend_class` 가 비어 있고 `fixed_cost=1` 이면 `essential` |
| 예산 사용 현황 `BudgetStatuses` (월 전용 예산이 기본 `*` 보다 우선, 주 예산은 부 지출까지 합산) | `budget.go:81`, `app.go:1111` | 뷰 `dash.budget_status_monthly` 가 이 결과와 같아야 한다 |
| 바구니 지출 처리 | `baskets_link.go:271 basketSpentIn` (`basket_entries.kind='spend'`, 금액은 음수로 저장), `budget.go` 의 `SUM(t.amount + COALESCE(be.amount,0))` | 대시보드 탭의 총지출에는 바구니 지출이 **포함**된다 (`Dashboard.tsx:249-251` "바구니에서 X · 평소 Y"). 예산 사용액에서는 **빠진다**. 낭비 계산도 예산과 같은 방식을 따른다 |
| 카테고리 편집 UI (고정비 체크) | `frontend/src/CategoryEditModal.tsx:27,43,57`, `frontend/src/pages/SettingsPage.tsx:563,622` (고정비 배지) | 같은 자리에 분류 선택을 추가한다 |
| 백업(SQLite 내보내기)의 카테고리 컬럼 목록 | `backup.go:21`, `backup.go:348-349` | **새 컬럼을 꼭 추가한다.** 백업은 내보내기만 하고 앱에 복원 기능은 없다. 빠뜨리면 백업 파일에 분류가 남지 않는다 |
| 스키마 지문 | `store.go:375 schemaStamp` (fullSchema 의 sha256), `store.go:369 fullSchema()` | 스키마 SQL 을 바꾸면 지문이 자동으로 바뀌고, 다음 시작 때 한 번만 적용된다 |

## 2. 작업 목록

### 작업 1. 스키마 — 컬럼

`baseSchema` 의 카테고리 쪽(`fixed_cost` 추가 줄 근처, `store.go:357` 부근)에 다음을 넣는다.

```sql
ALTER TABLE categories ADD COLUMN IF NOT EXISTS spend_class TEXT NOT NULL DEFAULT ''
  CHECK (spend_class IN ('', 'essential', 'discretionary', 'waste'));
-- '' = 미지정 | 'essential' = 필수 | 'discretionary' = 선택 | 'waste' = 낭비
```

- `ADD COLUMN IF NOT EXISTS` 는 컬럼이 이미 있으면 제약까지 통째로 건너뛰므로 여러 번 돌려도 안전하다. Go 쪽 setter 에서도 같은 값만 받는다.
- 한 번만 도는 기본 분류 시드: `seed.fixed_cost` 와 같은 패턴. `app_settings` 키는 `seed.spend_class`.
  - 주 카테고리 중 `spend_class=''` 인 것만 이름으로 채운다.
  - 기본 이름표: `통신비·대출상환·의료비·교육비 → essential`, `문화/여가 → discretionary`.
  - 실제 DB 의 카테고리 이름은 사용자가 바꿨을 수 있으므로, 없는 이름은 그냥 넘어간다.
  - **새 DB 에서는 스키마 시드가 기본 카테고리보다 먼저 돌아 아무것도 채우지 못한다** (`seed.fixed_cost` 도 같은 이유로 `store.go:431` 에서 따로 켠다). 기본 카테고리를 만드는 곳(`store.go:431` 옆)에서도 같은 기본 분류를 채운다.

### 작업 2. 모델과 조회

- `models.go` `Category` 에 다음을 추가한다:
  - `SpendClass string json:"spendClass"`
  - `ParentSpendClass string json:"parentSpendClass"`
  - 부 카테고리는 자기 값이 `''` 이면 주의 값을 쓴다. `ParentFixed` 와 같은 방식이다.
- `queries.go:51` 카테고리 목록 쿼리에 `c.spend_class`, `COALESCE(p.spend_class,'')` 를 추가한다.
- `queries.go:308` 옆에 `SetCategorySpendClass(id int64, v string) error` 를 추가한다. 허용 값 4개 외에는 오류를 돌려준다.
- `app.go:376` 옆에 `func (a *App) SetCategorySpendClass(id int64, v string) error` 를 추가한다.

### 작업 3. 집계 — `dash` 스키마 뷰 (대시보드와 앱이 같이 쓰는 계산)

`fullSchema()` **맨 끝**(모든 표와 컬럼 다음)에 `dashSchema` 를 붙인다.

```sql
CREATE SCHEMA IF NOT EXISTS dash;
-- 먼저 4개를 한꺼번에, 의존하는 쪽부터 지운다
DROP VIEW IF EXISTS dash.waste_top_monthly, dash.spending_health_monthly,
                    dash.budget_status_monthly, dash.month_summary;
-- 그다음 의존 순서대로 만든다
CREATE VIEW dash.month_summary AS ...;
CREATE VIEW dash.budget_status_monthly AS ...;
CREATE VIEW dash.spending_health_monthly AS ...;
CREATE VIEW dash.waste_top_monthly AS ...;
```

- 뷰를 지우고 다시 만드는 이유: `CREATE OR REPLACE VIEW` 는 컬럼을 빼거나 순서를 바꾸면 실패해서 앱 시작이 막힌다.
- **뷰마다 DROP→CREATE 를 번갈아 쓰면 안 된다.** 한 뷰가 다른 뷰를 참조하면(예: 낭비율 뷰가 예산 뷰를 참조) 두 번째 스키마 적용 때 "다른 객체가 의존한다" 오류로 DROP 이 실패한다. 스키마 적용(`migrate`)은 한 번의 실행이라 실패하면 앱이 켜지지 않는다.
- `DROP SCHEMA dash CASCADE` 도 쓰지 않는다. 스키마를 지우면 작업 4 의 `GRANT USAGE` 와 `ALTER DEFAULT PRIVILEGES` 가 함께 사라진다.
- 이전 버전 앱과의 호환: 지금 스키마에는 컬럼 타입을 바꾸는 문장(`ALTER COLUMN ... TYPE`)이 없어서 뷰가 있어도 이전 버전의 스키마 적용이 막히지 않는다. 앞으로 뷰가 참조하는 컬럼의 타입을 바꾸는 문장을 넣으면 이전 버전 앱이 시작하지 못하니 피한다.
- 다시 만들어도 권한은 유지된다. 아래 "1회 SQL" 의 `ALTER DEFAULT PRIVILEGES` 가 새 뷰에 자동으로 권한을 준다.
- 날짜는 TEXT `YYYY-MM-DD` 이므로 월은 `substr(t.date,1,7)` 로 묶는다. 거래가 있는 달마다 한 줄씩 나온다.
- 뷰는 소유자(앱이 접속하는 `postgres` 역할) 권한으로 실행된다. 그래서 `dashboard_ro` 는 원본 표 권한 없이 뷰만 읽을 수 있다. `security_invoker` 는 켜지 않는다.

만들 뷰 4개:

| 뷰 | 컬럼 | 같아야 하는 Go 결과 |
|---|---|---|
| `dash.month_summary` | `ym, income, expense, transfer, basket_expense, unclassified_count` | `MonthlySummary` 의 `TotalIncome/TotalExpense/TotalTransfer/BasketExpense/UnclassifiedCount` |
| `dash.budget_status_monthly` | `ym, category_id, category, amount, spent, remaining, over, override` | `BudgetStatuses(year, month)` |
| `dash.spending_health_monthly` | `ym, total_expense, essential, discretionary, waste, unassigned, basket, discretionary_over_budget, waste_amount, waste_rate, discretionary_rate, unassigned_rate` | 아래 규칙 |
| `dash.waste_top_monthly` | `ym, kind ('category'|'merchant'), name, amount, rank` | 낭비 카테고리·가맹점 상위 5 (`rank <= 5` 만) |

`spending_health_monthly` 계산 규칙:
- 거래 한 건의 분류 (위에서부터 먼저 맞는 것):
  1. 부 카테고리의 `spend_class`
  2. 주 카테고리의 `spend_class`
  3. `fixed_cost=1` (부 또는 주)이거나 정기결제에 연결된 거래(`subscription_posts`) → `essential`
  4. 그 밖 → `unassigned`

  정기결제 연결 판정은 `member_analysis.go` 의 `spendKind` 와 같은 조건을 쓴다.
- 분류별 금액은 예산과 같이 `t.amount + COALESCE(be.amount,0)` (be = `basket_entries`, `kind='spend'`)로 센다. 바구니에서 나간 몫은 미리 떼어 둔 돈이라 낭비로 보지 않고 `basket` 칸에 따로 둔다. 항상 다음 식이 성립해야 한다:

  `essential + discretionary + waste + unassigned + basket == total_expense`

- 예산 초과분은 0절의 기본안대로 센다. `dash.budget_status_monthly` 의 `spent` 를 그대로 쓰지 않는다(주에 부 지출이 합산돼 이중 계산이 된다).
  - 그달 유효 예산(월 전용 예산이 기본 `*` 보다 우선)은 `budget_status_monthly` 와 같은 규칙으로 고른다.
  - 분류가 `discretionary` 인 거래만 가장 가까운 예산 하나에 묶는다(부 예산이 있으면 부, 없으면 주).
  - 예산마다 `max(0, 묶인 금액 − 예산액)` 을 더한다. 금액은 위와 같이 바구니 몫을 뺀 `t.amount + COALESCE(be.amount,0)`.
- 테스트에 넣을 사례: '선택' 주 아래 '낭비' 부, 주·부 모두 예산, 주 예산 아래 '필수' 부. 셋 모두 `waste_amount ≤ total_expense` 이고 같은 돈이 두 번 들어가지 않아야 한다.
- `waste_amount = waste + discretionary_over_budget`, 각 비율은 `total_expense` 로 나눈다. 0 이면 0.

앱 쪽:
- store: `internal/store/spending_health.go` 에 `SpendingHealth(year, month int) (SpendingHealth, error)` 를 만든다.
  - 그달 행은 `dash.spending_health_monthly`, 최근 6개월 추이도 같은 뷰, 상위 목록은 `dash.waste_top_monthly` 에서 읽는다.
  - **Go 로 계산식을 다시 만들지 않는다.**
- app: `func (a *App) GetSpendingHealth(year, month int) (store.SpendingHealth, error)`. 다른 `Get*` 메서드처럼 `a.ensure()` 를 먼저 부른다.
- 반환 모양 (sobi 관례대로 camelCase):
  ```go
  type SpendingHealth struct {
      YM                      string        `json:"ym"`            // "2026-11"
      TotalExpense            int64         `json:"totalExpense"`
      Essential               int64         `json:"essential"`
      Discretionary           int64         `json:"discretionary"`
      Waste                   int64         `json:"waste"`
      Unassigned              int64         `json:"unassigned"`
      Basket                  int64         `json:"basket"`
      DiscretionaryOverBudget int64         `json:"discretionaryOverBudget"`
      WasteAmount             int64         `json:"wasteAmount"`
      WasteRate               float64       `json:"wasteRate"`     // 0~1
      DiscretionaryRate       float64       `json:"discretionaryRate"`
      UnassignedRate          float64       `json:"unassignedRate"`
      PrevWasteRate           float64       `json:"prevWasteRate"` // 전월
      Trend                   []WastePoint  `json:"trend"`         // 최근 6개월 (오래된 것부터)
      TopWaste                []NamedAmount `json:"topWaste"`
  }
  type WastePoint struct {
      YM        string  `json:"ym"`
      WasteRate float64 `json:"wasteRate"`
  }
  ```

### 작업 4. 읽기 전용 DB 역할 (사용자가 1회 실행)

비밀번호가 들어가므로 앱 마이그레이션에 넣지 않는다. 이 SQL 을 `docs/dashboard_ro.sql` 로 저장해 두고, 새 버전 앱을 한 번 켜서 `dash` 스키마가 생긴 뒤 **사용자가 Supabase SQL 편집기에서** 실행한다.

실제 SQL 은 `docs/dashboard_ro.sql` 에 있다. 요지:

```sql
CREATE SCHEMA IF NOT EXISTS dash;
CREATE ROLE dashboard_ro LOGIN PASSWORD '<openssl rand -hex 24 로 만든 값>';
GRANT USAGE ON SCHEMA dash TO dashboard_ro;
GRANT SELECT ON ALL TABLES IN SCHEMA dash TO dashboard_ro;
ALTER DEFAULT PRIVILEGES FOR ROLE postgres IN SCHEMA dash GRANT SELECT ON TABLES TO dashboard_ro;

-- 확인: dash_usage = true, view_read = true, raw_read = false
SELECT has_schema_privilege('dashboard_ro', 'dash', 'USAGE')               AS dash_usage,
       has_table_privilege('dashboard_ro', 'dash.month_summary', 'SELECT')  AS view_read,
       has_table_privilege('dashboard_ro', 'public.transactions', 'SELECT') AS raw_read;
```

- 처음 초안의 `SET ROLE dashboard_ro` 확인은 쓰지 않는다. Postgres 16 부터는 역할을 만든 `postgres`(Supabase 에서는 슈퍼유저가 아님)도 멤버십 없이는 SET ROLE 이 거부된다. 그러면 뒤의 SELECT 가 `postgres` 권한으로 돌아 "막혀 있음" 확인이 거짓으로 통과한다.
- 일부러 오류를 내는 확인(원본 표 SELECT → permission denied)도 쓰지 않는다. SQL Editor 는 스크립트를 한 번에 실행하므로 오류가 나면 `CREATE ROLE` 까지 취소된다.
- 2026-11-27 에 postgres:16 에서 슈퍼유저가 아닌 소유 역할로 확인했다: 한 번에 실행 성공, `dashboard_ro` 로 직접 접속하면 `dash.*` 는 읽히고 `public.transactions` 는 거부, 앱이 뷰를 다시 만든 뒤에도 읽힘.

- 대시보드 연결 문자열은 기존 `config.json` 의 풀러 호스트를 그대로 쓰고, 사용자명만 `dashboard_ro.<project-ref>` (Supavisor 형식)로 바꾼다. 실제로 연결해서 확인한다.
- **포트는 세션 모드 `5432` 로 한다.** `config.json` 은 트랜잭션 모드(`6543`)이고, 이 모드는 prepared statement 를 지원하지 않아 앱도 simple protocol 로 쓴다(`dsn.go:12`). 대시보드의 Rust `sqlx` 는 기본으로 prepared statement 를 쓰므로 6543 에서는 오류가 날 수 있다. 대시보드는 연결 1개를 1시간마다 쓰므로 세션 모드로 충분하다.
- `dash` 스키마는 Supabase API(PostgREST) 노출 목록에 넣지 않는다.

### 작업 5. 화면 (데스크톱)

- `CategoryEditModal.tsx`: 고정비 체크 아래에 "지출 성격" 4칸 선택을 추가한다 (미지정 / 필수 / 선택 / 낭비). 저장할 때 `SetCategorySpendClass` 를 부른다. 바인딩은 `wails generate module` 이나 `wails dev` 로 다시 만든다.
- `SettingsPage.tsx`: 고정비 배지 옆에 분류 배지를 단다 (563, 622행 근처). 색은 `docs/DESIGN_SYSTEM.md` 를 따른다.
- (선택) 대시보드 탭에 "이번 달 낭비율" 카드를 추가한다. 넣는다면 `GetDashboard` 에 합치지 말고 `GetSpendingHealth` 를 따로 부른다. `GetDashboard` 응답 모양은 모바일 PWA 도 쓰므로 바꾸지 않는다.
- (선택) `alerts.go` 에 낭비율 경고를 추가할 수 있다. 기준값은 설정으로 둔다.

### 작업 6. 테스트

- DB 테스트: 기존 `*_db_test.go` 처럼 `TEST_DATABASE_URL` 이 있을 때만 돈다 (테이블을 TRUNCATE 하므로 **실제 DB 를 가리키면 안 된다**). 확인할 것:
  - `dash.month_summary` 와 `MonthlySummary` 가 같은 달에서 같다
  - `dash.budget_status_monthly` 와 `BudgetStatuses` 가 같다 (월 전용 예산, 주·부 합산, 바구니 포함 사례)
  - 합계 등식 (`essential + … + basket == total_expense`)
  - 분류 우선순위, 이중 계산 금지, 총지출 0 인 달
  - 스키마를 두 번 적용해도 실패하지 않는다 (뷰 DROP/CREATE)
- 백업 테스트(`backup_test.go`)에서 백업 파일에 `spend_class` 가 들어가는지 확인한다.
- `dash.budget_status_monthly` 와 `BudgetStatuses` 는 같은 계산이 Go 와 SQL 두 곳에 있는 셈이다. 같음 테스트가 둘이 어긋나는 것을 잡는 유일한 장치이므로 빼지 않는다.

### 작업 7. 배포

- `scripts/release.sh <버전>` → 태그 → GitHub Release (README "새 버전 배포" 절차).
- 새 버전 앱을 한 번 켜면 `dash` 뷰가 생긴다. 그다음 작업 4 의 SQL 을 실행하고, 연결 문자열을 대시보드 설정(macOS 키체인 `sobi.db_url`)에 넣는다.

## 3. 완료 기준

- [ ] `go test ./...` 통과 (DB 테스트는 테스트 DB 로)
- [ ] 카테고리 편집에서 분류를 바꾸면 저장되고, 앱을 다시 켜도 유지된다
- [ ] 백업 파일(SQLite)에 분류가 들어간다
- [ ] 새 DB(테스트 DB)에서도 기본 분류가 채워진다
- [ ] 스키마를 두 번 적용해도(뷰 다시 만들기) 앱이 정상 시작한다
- [ ] 같은 달의 `GetSpendingHealth().totalExpense` 와 대시보드 탭의 총지출이 같다
- [ ] `dashboard_ro` 로 `dash.*` 는 읽히고 `public.*` 는 permission denied
- [ ] 이전 버전 앱(v1.11.x)으로 같은 DB 를 열어도 문제없다 (컬럼·뷰 추가만 하므로)

## 4. 범위 밖

- 거래 단위 낭비 표시 (2단계)
- "안 쓰는 구독" 자동 판정. 결제 기록만으로는 사용 여부를 알 수 없다.
- 모바일 PWA(`sobi-master-mobile`) 변경
- 대시보드 본체 코드
