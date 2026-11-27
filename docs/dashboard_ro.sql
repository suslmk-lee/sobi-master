-- 개인 대시보드용 읽기 전용 역할 (사용자가 한 번만 실행)
--
-- 언제: 이 기능이 들어간 새 버전 앱을 한 번 켜서 dash 스키마의 뷰가 생긴 뒤.
-- 어디서: Supabase 대시보드 → SQL Editor.
-- 비밀번호가 들어가므로 앱 마이그레이션에 넣지 않는다. 아래 '<비밀번호>' 를
--   openssl rand -hex 24
-- 로 만든 값으로 바꿔서 실행하고, 그 값은 이 파일에 저장하지 않는다.
--
-- dashboard_ro 는 dash 스키마의 뷰만 읽는다. 뷰는 소유자(앱이 접속하는 postgres) 권한으로 원본 표를 읽으므로
-- 원본 표(public.*) 권한은 주지 않는다. 앱이 스키마를 다시 적용하며 뷰를 새로 만들어도
-- ALTER DEFAULT PRIVILEGES 덕분에 읽기 권한이 자동으로 붙는다.

CREATE SCHEMA IF NOT EXISTS dash;
CREATE ROLE dashboard_ro LOGIN PASSWORD '<비밀번호>';
GRANT USAGE ON SCHEMA dash TO dashboard_ro;
GRANT SELECT ON ALL TABLES IN SCHEMA dash TO dashboard_ro;
ALTER DEFAULT PRIVILEGES FOR ROLE postgres IN SCHEMA dash GRANT SELECT ON TABLES TO dashboard_ro;

-- 확인: 결과가 dash_usage = true, view_read = true, raw_read = false 여야 한다.
-- 일부러 오류를 내는 확인(원본 표 SELECT → permission denied)은 쓰지 않는다. SQL Editor 는 스크립트를 한 번에
-- 실행하므로 오류가 나면 위의 CREATE ROLE 까지 함께 취소된다. SET ROLE 도 쓰지 않는다 — Postgres 16 부터는
-- 역할을 만든 postgres(슈퍼유저 아님)도 멤버십 없이는 SET ROLE 이 거부된다.
SELECT has_schema_privilege('dashboard_ro', 'dash', 'USAGE')               AS dash_usage,
       has_table_privilege('dashboard_ro', 'dash.month_summary', 'SELECT')  AS view_read,
       has_table_privilege('dashboard_ro', 'public.transactions', 'SELECT') AS raw_read;

-- 가장 확실한 확인은 이 역할로 직접 접속해 보는 것이다 (아래 연결 문자열로 dash.* 는 읽히고 public.* 는 거부).
--
-- 연결 문자열 (대시보드 설정의 macOS 키체인 항목 sobi.db_url 에 넣는다):
--   기존 config.json 의 풀러 호스트를 그대로 쓰고
--   - 사용자명: dashboard_ro.<project-ref>   (Supavisor 형식)
--   - 포트: 5432 (세션 모드). 6543(트랜잭션 모드)은 prepared statement 를 지원하지 않아
--     Rust sqlx 기본 설정으로는 오류가 날 수 있다.
-- dash 스키마는 Supabase API(PostgREST) 노출 목록에 넣지 않는다.
