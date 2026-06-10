package store

import (
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// normalizeDSN 은 사용자가 붙여 넣은 연결 문자열을 손질한다.
//   - 비밀번호에 특수문자(*, :, @, / 등)가 있어 URL 파싱이 깨지면 자동으로 percent-encoding 한다.
//   - Supabase Transaction pooler(6543)는 prepared statement 를 지원하지 않으므로
//     실행 모드를 simple protocol 로 고정한다.
func normalizeDSN(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if _, err := pgconn.ParseConfig(dsn); err != nil {
		if fixed := encodeUserinfo(dsn); fixed != dsn {
			if _, err := pgconn.ParseConfig(fixed); err == nil {
				dsn = fixed
			}
		}
	}
	// URL 형식이고 실행 모드 미지정이면 pooler 호환 모드 추가
	if strings.Contains(dsn, "://") && !strings.Contains(dsn, "default_query_exec_mode") {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		dsn += sep + "default_query_exec_mode=simple_protocol"
	}
	return dsn
}

// encodeUserinfo 는 URL 의 사용자:비밀번호 부분을 percent-encoding 해 다시 조립한다.
func encodeUserinfo(dsn string) string {
	i := strings.Index(dsn, "://")
	if i < 0 {
		return dsn
	}
	scheme, rest := dsn[:i+3], dsn[i+3:]
	at := strings.LastIndex(rest, "@")
	if at < 0 {
		return dsn
	}
	userinfo, host := rest[:at], rest[at+1:]
	user := userinfo
	pass := ""
	hasPass := false
	if c := strings.Index(userinfo, ":"); c >= 0 {
		user, pass, hasPass = userinfo[:c], userinfo[c+1:], true
	}
	// 이미 인코딩돼 있을 수 있으니 디코드 시도 후 다시 인코딩 (이중 인코딩 방지)
	if du, err := url.QueryUnescape(user); err == nil {
		user = du
	}
	if dp, err := url.QueryUnescape(pass); err == nil {
		pass = dp
	}
	var ui *url.Userinfo
	if hasPass {
		ui = url.UserPassword(user, pass)
	} else {
		ui = url.User(user)
	}
	return scheme + ui.String() + "@" + host
}
