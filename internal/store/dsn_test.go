package store

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestNormalizeDSN(t *testing.T) {
	cases := []struct {
		name string
		dsn  string
	}{
		{"특수문자 비밀번호 (*)", "postgresql://postgres.abc:mTk*US8n@aws-1.pooler.supabase.com:6543/postgres"},
		{"콜론 포함 비밀번호", "postgresql://postgres.abc:pa:ss@aws-1.pooler.supabase.com:5432/postgres"},
		{"골뱅이 포함 비밀번호", "postgresql://postgres.abc:p@ss!@aws-1.pooler.supabase.com:5432/postgres"},
		{"정상 URL", "postgresql://postgres:simple@db.host.com:5432/postgres"},
		{"이미 인코딩된 비밀번호", "postgresql://postgres:p%40ss@db.host.com:5432/postgres"},
	}
	for _, c := range cases {
		out := normalizeDSN(c.dsn)
		cfg, err := pgconn.ParseConfig(out)
		if err != nil {
			t.Errorf("%s: 정규화 후에도 파싱 실패: %v", c.name, err)
			continue
		}
		if cfg.Host == "" {
			t.Errorf("%s: 호스트 비어 있음", c.name)
		}
		if !strings.Contains(out, "default_query_exec_mode=simple_protocol") {
			t.Errorf("%s: pooler 호환 모드 누락", c.name)
		}
	}

	// 비밀번호가 원본 그대로 복원되는지 (이중 인코딩 없음)
	out := normalizeDSN("postgresql://u:p%40ss@h:5432/db")
	cfg, _ := pgconn.ParseConfig(out)
	if cfg.Password != "p@ss" {
		t.Errorf("이중 인코딩 발생: %q", cfg.Password)
	}
	out2 := normalizeDSN("postgresql://u:mTk*US8n@h:5432/db")
	cfg2, _ := pgconn.ParseConfig(out2)
	if cfg2.Password != "mTk*US8n" {
		t.Errorf("비밀번호 손상: %q", cfg2.Password)
	}
}
