package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config 는 <설정 디렉토리>/sobi/config.json 에 저장된다.
// DatabaseURL 에는 Supabase 의 Postgres 연결 문자열을 넣는다.
// (Supabase 대시보드 → Connect → Session pooler URI 권장)
type Config struct {
	DatabaseURL string `json:"database_url"`
}

func configDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "sobi")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func ConfigPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// LogPath 는 오류 로그 파일 경로. (macOS: ~/Library/Application Support/sobi/sobi.log,
// Windows: %AppData%\sobi\sobi.log)
func LogPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "sobi.log"), nil
}

// LegacySQLitePath 는 이전 버전이 쓰던 로컬 SQLite 파일 경로.
func LegacySQLitePath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "sobi.db"), nil
}

// BackupDir 는 로컬 SQLite 백업 파일들이 쌓이는 디렉토리 (<설정 디렉토리>/backups).
func BackupDir() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	bdir := filepath.Join(dir, "backups")
	if err := os.MkdirAll(bdir, 0o755); err != nil {
		return "", err
	}
	return bdir, nil
}

// LoadConfig 는 설정 파일을 읽는다. 파일이 없으면 빈 템플릿을 만들어 두고
// 사용자가 채워 넣도록 안내하는 에러를 돌려준다.
func LoadConfig() (Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return Config{}, err
	}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		tpl, _ := json.MarshalIndent(Config{DatabaseURL: ""}, "", "  ")
		if werr := os.WriteFile(path, tpl, 0o600); werr != nil {
			return Config{}, werr
		}
		return Config{}, fmt.Errorf("Supabase 연결 정보가 없습니다.\n\n%s 파일의 database_url 에\nSupabase 대시보드 → Connect → Session pooler 연결 문자열을 넣고\n앱을 다시 실행하세요", path)
	}
	if err != nil {
		return Config{}, err
	}
	// Windows 메모장이 붙이는 UTF-8 BOM 허용
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("%s 파싱 실패: %w", path, err)
	}
	if cfg.DatabaseURL == "" {
		return cfg, fmt.Errorf("Supabase 연결 정보가 비어 있습니다.\n\n%s 파일의 database_url 에\nSupabase 대시보드 → Connect → Session pooler 연결 문자열을 넣고\n앱을 다시 실행하세요", path)
	}
	return cfg, nil
}
