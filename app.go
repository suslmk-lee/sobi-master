package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"sobi/internal/classifier"
	"sobi/internal/importer"
	"sobi/internal/store"
)

// App 은 Wails 바인딩 진입점. 프론트엔드에서 호출하는 모든 메서드가 여기 붙는다.
type App struct {
	ctx context.Context
	mu  sync.Mutex
	st  *store.Store
	cl  *classifier.Classifier
	// lastDeleted 는 일괄/단건 삭제 직후 복원(Undo)을 위한 백업.
	lastDeleted []store.Transaction
	// backupMu 는 로컬 백업을 한 번에 하나만 수행하도록 직렬화한다.
	backupMu   sync.Mutex
	backupDone chan struct{}
}

// backupInterval 는 앱 실행 중 자동 로컬 백업 주기.
const backupInterval = 6 * time.Hour

// backupKeep 는 보관할 백업 파일 개수(오래된 것부터 삭제).
const backupKeep = 7

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// 오류를 설정 디렉토리의 sobi.log 에 기록한다 (Windows: %AppData%\sobi\sobi.log)
	if path, err := store.LogPath(); err == nil {
		if f, ferr := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); ferr == nil {
			log.SetOutput(f)
		}
	}
	log.Printf("앱 시작")
	if err := a.ensure(); err != nil {
		// 종료하지 않는다 — 화면에 오류가 표시되고 다음 호출에서 자동 재시도된다
		log.Printf("시작 시 DB 연결 실패: %v", err)
	}
	a.startBackupLoop()
}

// startBackupLoop 은 시작 직후 한 번, 이후 backupInterval 마다 로컬 백업을 수행한다.
func (a *App) startBackupLoop() {
	a.backupDone = make(chan struct{})
	go func() {
		// 시작 직후 UI 를 막지 않도록 잠깐 뒤에 첫 백업
		select {
		case <-time.After(5 * time.Second):
		case <-a.backupDone:
			return
		}
		if _, err := a.doBackup(); err != nil {
			logIf("자동 백업(시작)", err)
		}
		t := time.NewTicker(backupInterval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				if _, err := a.doBackup(); err != nil {
					logIf("자동 백업(주기)", err)
				}
			case <-a.backupDone:
				return
			}
		}
	}()
}

// ensure 는 DB 연결을 보장한다. 시작 시 실패했어도 호출 시마다 다시 시도하므로
// 네트워크가 복구되거나 설정을 고치면 앱 재시작 없이도 동작한다.
func (a *App) ensure() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.st != nil {
		return nil
	}
	st, err := store.Open()
	if err != nil {
		return err
	}
	a.st = st
	a.cl = classifier.New(st)
	return nil
}

func (a *App) shutdown(ctx context.Context) {
	// 백업 루프를 멈추고, 종료 전 마지막 스냅샷을 남긴다.
	if a.backupDone != nil {
		close(a.backupDone)
	}
	if _, err := a.doBackup(); err != nil {
		logIf("자동 백업(종료)", err)
	}
	// backupMu 를 잡아 진행 중 백업이 끝난 뒤 연결을 닫는다(백업 중 Close 방지).
	a.backupMu.Lock()
	defer a.backupMu.Unlock()
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.st != nil {
		a.st.Close()
		a.st = nil
	}
}

func logIf(op string, err error) {
	if err != nil {
		log.Printf("%s: %v", op, err)
	}
}

// ---- 로컬 백업 ----

// BackupInfo 는 로컬 백업 한 건의 요약(경로/시각/크기/테이블별 행 수).
type BackupInfo struct {
	Path   string         `json:"path"`
	Time   string         `json:"time"` // 파일 수정 시각 "2006-01-02 15:04:05"
	SizeKB int64          `json:"sizeKB"`
	Counts map[string]int `json:"counts"`
}

func backupInfoOf(path string, counts map[string]int) BackupInfo {
	info := BackupInfo{Path: path, Counts: counts}
	if fi, err := os.Stat(path); err == nil {
		info.Time = fi.ModTime().Format("2006-01-02 15:04:05")
		info.SizeKB = fi.Size() / 1024
	}
	return info
}

// doBackup 은 현재 연결된 DB 를 로컬 SQLite 로 스냅샷 백업한다(직렬화, 재연결하지 않음).
// 연결이 없으면 건너뛴다. 종료/주기/수동 백업이 공유한다.
func (a *App) doBackup() (BackupInfo, error) {
	a.backupMu.Lock()
	defer a.backupMu.Unlock()
	a.mu.Lock()
	st := a.st
	a.mu.Unlock()
	if st == nil {
		return BackupInfo{}, fmt.Errorf("DB 미연결 — 백업을 건너뜁니다")
	}
	dir, err := store.BackupDir()
	if err != nil {
		return BackupInfo{}, err
	}
	p := filepath.Join(dir, "sobi_"+time.Now().Format("20060102_150405")+".db")
	counts, err := st.BackupToSQLite(p)
	if err != nil {
		return BackupInfo{}, err
	}
	a.pruneBackups(dir, backupKeep)
	log.Printf("로컬 백업 완료: %s (%v)", p, counts)
	return backupInfoOf(p, counts), nil
}

// pruneBackups 는 백업 파일이 keep 개를 넘으면 오래된 것부터 지운다.
// 파일명이 타임스탬프라 사전순 정렬 = 시간순.
func (a *App) pruneBackups(dir string, keep int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	names := []string{}
	for _, e := range entries {
		n := e.Name()
		if strings.HasPrefix(n, "sobi_") && strings.HasSuffix(n, ".db") {
			names = append(names, n)
		}
	}
	if len(names) <= keep {
		return
	}
	sort.Strings(names)
	for _, n := range names[:len(names)-keep] {
		_ = os.Remove(filepath.Join(dir, n))
	}
}

// BackupNow 는 사용자가 즉시 백업을 요청할 때 호출된다(필요 시 연결부터 시도).
func (a *App) BackupNow() (BackupInfo, error) {
	if err := a.ensure(); err != nil {
		return BackupInfo{}, err
	}
	return a.doBackup()
}

// GetLastBackup 은 가장 최근 백업 파일의 정보를 돌려준다(없으면 Path 빈 값).
func (a *App) GetLastBackup() (BackupInfo, error) {
	dir, err := store.BackupDir()
	if err != nil {
		return BackupInfo{}, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return BackupInfo{}, err
	}
	latest := ""
	for _, e := range entries {
		n := e.Name()
		if strings.HasPrefix(n, "sobi_") && strings.HasSuffix(n, ".db") && n > latest {
			latest = n
		}
	}
	if latest == "" {
		return BackupInfo{}, nil
	}
	return backupInfoOf(filepath.Join(dir, latest), nil), nil
}

// ---- 귀속자 / 카테고리 / 결제수단 ----

func (a *App) ListMembers() ([]store.Member, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return a.st.ListMembers()
}

func (a *App) AddMember(name string) (store.Member, error) {
	if err := a.ensure(); err != nil {
		return store.Member{}, err
	}
	m, err := a.st.AddMember(name)
	logIf("AddMember", err)
	return m, err
}

func (a *App) DeleteMember(id int64) error {
	if err := a.ensure(); err != nil {
		return err
	}
	return a.st.DeleteMember(id)
}

func (a *App) ListCategories() ([]store.Category, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return a.st.ListCategories()
}

// AddCategory 는 카테고리를 추가한다. parentID>0 이면 그 주 카테고리의 부로 만든다
// (부의 수입/지출/이체 종류는 주를 따른다).
func (a *App) AddCategory(name, kind string, parentID int64) (store.Category, error) {
	if err := a.ensure(); err != nil {
		return store.Category{}, err
	}
	c, err := a.st.AddCategory(name, kind, parentID)
	logIf("AddCategory", err)
	return c, err
}

// SetCategoryParent 는 카테고리의 상위(주)를 바꾼다. parentID=0 이면 주로 승격.
func (a *App) SetCategoryParent(id, parentID int64) error {
	if err := a.ensure(); err != nil {
		return err
	}
	err := a.st.SetCategoryParent(id, parentID)
	logIf("SetCategoryParent", err)
	return err
}

func (a *App) DeleteCategory(id int64) error {
	if err := a.ensure(); err != nil {
		return err
	}
	return a.st.DeleteCategory(id)
}

func (a *App) ListPaymentMethods() ([]store.PaymentMethod, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return a.st.ListPaymentMethods()
}

func (a *App) AddPaymentMethod(name, typ string) (store.PaymentMethod, error) {
	if err := a.ensure(); err != nil {
		return store.PaymentMethod{}, err
	}
	p, err := a.st.AddPaymentMethod(name, typ)
	logIf("AddPaymentMethod", err)
	return p, err
}

func (a *App) DeletePaymentMethod(id int64) error {
	if err := a.ensure(); err != nil {
		return err
	}
	return a.st.DeletePaymentMethod(id)
}

// ---- 거래 ----

func (a *App) ListTransactions(filter store.TxFilter) ([]store.Transaction, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return a.st.ListTransactions(filter)
}

// GetMerchantSuggestions 는 수동 등록 내용 칸 자동완성용: query 를 포함하는 과거 가맹점과
// 그 가맹점의 최근 거래 정보(메모/귀속자/카테고리/결제수단/구분)를 최근순으로 돌려준다.
func (a *App) GetMerchantSuggestions(query string) ([]store.MerchantSuggestion, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(query) == "" {
		return []store.MerchantSuggestion{}, nil
	}
	return a.st.MerchantSuggestions(query, 8)
}

// AddTransaction 은 수동 등록. 학습된 규칙(가맹점+금액)이 있으면 규칙이 우선하여
// 폼 기본값(기본 귀속자 등)을 덮어쓰고 자동 분류한다. 이미 있는 규칙이므로 다시
// 학습하지 않아 규칙이 오염되지 않는다. 규칙이 없을 때는 폼 값 그대로 저장하고,
// 사용자가 카테고리까지 골라 직접 분류한 경우에만 그 분류를 규칙으로 학습한다.
// (귀속자는 기본값이 채워질 수 있어 단독으로는 학습 신호로 보지 않는다.)
func (a *App) AddTransaction(t store.Transaction) (int64, error) {
	if err := a.ensure(); err != nil {
		return 0, err
	}
	if t.Date == "" || t.Amount <= 0 {
		return 0, fmt.Errorf("날짜와 금액(양수)은 필수입니다")
	}
	t.Source = "manual"

	rule, err := a.cl.Match(t.Merchant, t.Amount)
	if err != nil {
		logIf("AddTransaction(분류)", err)
		return 0, err
	}
	matched := rule != nil
	if matched {
		// 규칙 우선: 폼 기본값을 무시하고 규칙의 분류로 덮어쓴다.
		if rule.MemberID != nil {
			id := *rule.MemberID
			t.MemberID = &id
		}
		if rule.CategoryID != nil {
			id := *rule.CategoryID
			t.CategoryID = &id
		}
		t.AutoClassified = true
	}

	id, err := a.st.AddTransaction(t)
	if err != nil {
		logIf("AddTransaction", err)
		return 0, err
	}
	if !matched && t.CategoryID != nil {
		a.learn(t)
	}
	return id, nil
}

// ClassifyTransaction 은 사용자가 거래의 분류(귀속자/카테고리/결제수단)를 확정할 때
// 호출된다. 거래를 갱신하고 핑거프린트 규칙을 학습해 다음 달부터 자동 분류한다.
func (a *App) ClassifyTransaction(t store.Transaction) error {
	if err := a.ensure(); err != nil {
		return err
	}
	t.AutoClassified = false
	if err := a.st.UpdateTransaction(t); err != nil {
		logIf("ClassifyTransaction", err)
		return err
	}
	a.learn(t)
	return nil
}

func (a *App) UpdateTransaction(t store.Transaction) error {
	if err := a.ensure(); err != nil {
		return err
	}
	err := a.st.UpdateTransaction(t)
	logIf("UpdateTransaction", err)
	return err
}

func (a *App) DeleteTransaction(id int64) error {
	if err := a.ensure(); err != nil {
		return err
	}
	if t, err := a.st.GetTransaction(id); err == nil {
		a.mu.Lock()
		a.lastDeleted = []store.Transaction{t}
		a.mu.Unlock()
	}
	return a.st.DeleteTransaction(id)
}

// BatchDelete 는 여러 거래를 한 번에 삭제하고 복원용 백업을 남긴다.
func (a *App) BatchDelete(ids []int64) error {
	if err := a.ensure(); err != nil {
		return err
	}
	backup := make([]store.Transaction, 0, len(ids))
	for _, id := range ids {
		if t, err := a.st.GetTransaction(id); err == nil {
			backup = append(backup, t)
		}
	}
	if err := a.st.DeleteTransactions(ids); err != nil {
		return err
	}
	a.mu.Lock()
	a.lastDeleted = backup
	a.mu.Unlock()
	return nil
}

// UndoDelete 는 마지막 삭제(단건/일괄)를 되돌린다. 새 ID로 다시 추가된다.
func (a *App) UndoDelete() (int, error) {
	if err := a.ensure(); err != nil {
		return 0, err
	}
	a.mu.Lock()
	backup := a.lastDeleted
	a.lastDeleted = nil
	a.mu.Unlock()
	n := 0
	for _, t := range backup {
		t.ID = 0
		if _, err := a.st.AddTransaction(t); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// CanUndo 는 되돌릴 삭제가 있는지 본다.
func (a *App) CanUndo() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.lastDeleted) > 0
}

// BatchClassify 는 여러 거래의 귀속자/카테고리를 한 번에 지정한다(0은 변경 안 함).
// learn 이 true 일 때만 분류 규칙을 학습한다(대량 처리 시 규칙 오염 방지를 위해 기본 false).
func (a *App) BatchClassify(ids []int64, memberID, categoryID int64, learn bool) error {
	if err := a.ensure(); err != nil {
		return err
	}
	for _, id := range ids {
		t, err := a.st.GetTransaction(id)
		if err != nil {
			continue
		}
		if memberID > 0 {
			m := memberID
			t.MemberID = &m
		}
		if categoryID > 0 {
			c := categoryID
			t.CategoryID = &c
		}
		t.AutoClassified = false
		if err := a.st.UpdateTransaction(t); err != nil {
			return err
		}
		if learn {
			a.learn(t)
		}
	}
	return nil
}

// ApplyRulesToUnclassified 는 학습된 규칙을 미분류 거래에 소급 적용한다.
// month 가 빈 문자열이면 전체. 적용된 건수를 돌려준다.
func (a *App) ApplyRulesToUnclassified(month string) (int, error) {
	if err := a.ensure(); err != nil {
		return 0, err
	}
	txs, err := a.st.ListTransactions(store.TxFilter{Month: month, UnclassifiedOnly: true})
	if err != nil {
		return 0, err
	}
	applied := 0
	for _, t := range txs {
		rule, err := a.cl.Match(t.Merchant, t.Amount)
		if err != nil {
			return applied, err
		}
		if rule == nil {
			continue
		}
		t.AutoClassified = false
		classifier.Apply(&t, rule) // 빈 필드만 채움
		if t.AutoClassified {
			if err := a.st.UpdateTransaction(t); err != nil {
				return applied, err
			}
			applied++
		}
	}
	return applied, nil
}

// learn 은 규칙 라벨에 쓸 이름을 채운 뒤 분류 규칙을 학습한다. 실패해도 거래 저장은 유지.
func (a *App) learn(t store.Transaction) {
	if t.MemberID != nil && t.MemberName == "" {
		if ms, err := a.st.ListMembers(); err == nil {
			for _, m := range ms {
				if m.ID == *t.MemberID {
					t.MemberName = m.Name
				}
			}
		}
	}
	if t.CategoryID != nil && t.CategoryName == "" {
		if cs, err := a.st.ListCategories(); err == nil {
			for _, c := range cs {
				if c.ID == *t.CategoryID {
					t.CategoryName = c.Name
				}
			}
		}
	}
	logIf("규칙 학습", a.cl.Learn(t))
}

// ---- CSV 가져오기 ----

type ImportResult struct {
	File           string   `json:"file"`
	Total          int      `json:"total"`
	Imported       int      `json:"imported"`
	AutoClassified int      `json:"autoClassified"`
	Duplicates     int      `json:"duplicates"`
	Errors         []string `json:"errors"`
}

// ImportCSV 는 파일 선택 대화상자를 열어 카드사/은행 CSV 를 읽고,
// 중복(같은 날짜+금액+가맹점)을 건너뛰면서 거래를 등록한다.
// 등록 시 규칙이 맞으면 자동 분류까지 수행한다. 결제수단을 지정하면 모든 행에 적용.
func (a *App) ImportCSV(paymentMethodID int64) (ImportResult, error) {
	if err := a.ensure(); err != nil {
		return ImportResult{}, err
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "카드/은행 이용내역 CSV 선택",
		Filters: []runtime.FileFilter{
			{DisplayName: "CSV 파일 (*.csv)", Pattern: "*.csv;*.CSV"},
		},
	})
	if err != nil {
		return ImportResult{}, err
	}
	if path == "" { // 사용자가 취소
		return ImportResult{}, nil
	}

	parsed, err := importer.ParseFile(path)
	if err != nil {
		logIf("ImportCSV(파싱)", err)
		return ImportResult{File: path}, err
	}

	res := ImportResult{File: path, Total: parsed.Total, Errors: parsed.Errors}
	var pmID *int64
	if paymentMethodID > 0 {
		pmID = &paymentMethodID
	}
	for _, row := range parsed.Parsed {
		dup, err := a.st.HasTransaction(row.Date, row.Amount, row.Merchant, row.Direction)
		if err != nil {
			logIf("ImportCSV(중복확인)", err)
			return res, err
		}
		if dup {
			res.Duplicates++
			continue
		}
		t := row.ToTransaction()
		t.PaymentMethodID = pmID
		rule, err := a.cl.Match(t.Merchant, t.Amount)
		if err != nil {
			return res, err
		}
		classifier.Apply(&t, rule)
		if t.AutoClassified {
			res.AutoClassified++
		}
		if _, err := a.st.AddTransaction(t); err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s %s: %v", row.Date, row.Merchant, err))
			continue
		}
		res.Imported++
	}
	return res, nil
}

// ---- 카드 ----

// SaveCard 는 카드 등록/수정. ID 가 0이면 새 카드.
func (a *App) SaveCard(p store.PaymentMethod) (store.PaymentMethod, error) {
	if err := a.ensure(); err != nil {
		return p, err
	}
	if p.Name == "" {
		return p, fmt.Errorf("카드 이름은 필수입니다")
	}
	p.Type = "card"
	saved, err := a.st.SavePaymentMethod(p)
	logIf("SaveCard", err)
	return saved, err
}

// GetCardStatuses 는 모든 카드의 현재 실적기간 사용액/한도 달성 현황.
func (a *App) GetCardStatuses() ([]store.CardStatus, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return a.st.CardStatuses(time.Now())
}

// GetCardBreakdown 은 카드 한 장의 기간 내 가맹점별/카테고리별 지출 분석.
func (a *App) GetCardBreakdown(pmID int64, start, end string) (store.CardBreakdown, error) {
	if err := a.ensure(); err != nil {
		return store.CardBreakdown{}, err
	}
	return a.st.CardBreakdown(pmID, start, end)
}

// ---- 규칙 / 집계 ----

func (a *App) ListRules() ([]store.Rule, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return a.st.ListRules()
}

func (a *App) DeleteRule(id int64) error {
	if err := a.ensure(); err != nil {
		return err
	}
	return a.st.DeleteRule(id)
}

func (a *App) GetMonthlySummary(year, month int) (store.MonthlySummary, error) {
	if err := a.ensure(); err != nil {
		return store.MonthlySummary{}, err
	}
	return a.st.MonthlySummary(year, month)
}

// GetMonthlyTrend 는 (year, month) 까지 최근 n개월의 수입/지출/이체 추이.
func (a *App) GetMonthlyTrend(year, month, n int) ([]store.MonthPoint, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return a.st.MonthlyTrend(year, month, n)
}

// GetDailyExpenses 는 해당 월의 일별 지출 합계.
func (a *App) GetDailyExpenses(year, month int) ([]store.DayPoint, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return a.st.DailyExpenses(year, month)
}

// GetTopMerchants 는 해당 월 지출 상위 가맹점.
func (a *App) GetTopMerchants(year, month, limit int) ([]store.NamedAmount, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return a.st.TopMerchants(year, month, limit)
}

// ---- 통계 ----

// GetDailyByDimension 은 해당 월의 일별 지출을 차원(paymentMethod|member|category)별로 분해한다.
func (a *App) GetDailyByDimension(year, month int, dimension string) (store.DailyByDimension, error) {
	if err := a.ensure(); err != nil {
		return store.DailyByDimension{}, err
	}
	return a.st.DailyByDimension(year, month, dimension)
}

// GetCardPaces 는 실적 목표가 설정된 카드의 현재 실적기간 누적 진행 현황.
func (a *App) GetCardPaces() ([]store.CardPace, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return a.st.CardPaces(time.Now())
}

// GetCardCategoryBreakdown 은 해당 월의 결제수단별 카테고리 지출 구성.
func (a *App) GetCardCategoryBreakdown(year, month int) ([]store.CardCategory, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return a.st.CardCategoryBreakdown(year, month)
}

// GetMemberCategoryBreakdown 은 해당 월의 귀속자별 카테고리 지출 구성.
func (a *App) GetMemberCategoryBreakdown(year, month int) ([]store.CardCategory, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return a.st.MemberCategoryBreakdown(year, month)
}

// GetMemberTrend 는 귀속자별 최근 n개월 지출 추이.
func (a *App) GetMemberTrend(year, month, n int) (store.CategoryTrend, error) {
	if err := a.ensure(); err != nil {
		return store.CategoryTrend{}, err
	}
	return a.st.MemberTrend(year, month, n)
}

// GetMemberStats 는 귀속자별 지출 요약(총액·비중·전월 대비·최다 카테고리/가맹점).
func (a *App) GetMemberStats(year, month int) ([]store.MemberStat, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return a.st.MemberStats(year, month)
}

// GetCategoryMemberBreakdown 은 해당 월의 카테고리별 귀속자 지출 구성. level 은 main|sub.
func (a *App) GetCategoryMemberBreakdown(year, month int, level string) ([]store.CardCategory, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return a.st.CategoryMemberBreakdown(year, month, level)
}

// GetCategoryMatrix 는 카테고리×월 히트맵용: 전체 카테고리의 최근 n개월 월별 지출. level 은 main|sub.
func (a *App) GetCategoryMatrix(year, month, n int, level string) (store.CategoryTrend, error) {
	if err := a.ensure(); err != nil {
		return store.CategoryTrend{}, err
	}
	return a.st.CategoryMatrix(year, month, n, level)
}

// GetCategoryDetail 은 카테고리 한 개의 상세 분석. period 는 month|prev|recent30.
func (a *App) GetCategoryDetail(year, month int, categoryID int64, period string) (store.CategoryDetail, error) {
	if err := a.ensure(); err != nil {
		return store.CategoryDetail{}, err
	}
	return a.st.CategoryDetail(year, month, categoryID, period, time.Now(), 6)
}

// GetYearSummary 는 연간 보기(12개월 추이 + 전년 대비 + 카테고리).
func (a *App) GetYearSummary(year int) (store.YearSummary, error) {
	if err := a.ensure(); err != nil {
		return store.YearSummary{}, err
	}
	return a.st.YearSummary(year)
}

// ---- 예산 ----

// ListBudgets 는 ym("*"=매월 기본, "YYYY-MM"=해당 월 전용)으로 설정된 예산 목록.
func (a *App) ListBudgets(ym string) ([]store.Budget, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return a.st.ListBudgets(ym)
}

// SetBudget 은 (카테고리, ym) 예산을 저장한다. amount<=0 이면 해제.
func (a *App) SetBudget(categoryID int64, ym string, amount int64) error {
	if err := a.ensure(); err != nil {
		return err
	}
	err := a.st.SetBudget(categoryID, ym, amount)
	logIf("SetBudget", err)
	return err
}

func (a *App) DeleteBudget(categoryID int64, ym string) error {
	if err := a.ensure(); err != nil {
		return err
	}
	return a.st.DeleteBudget(categoryID, ym)
}

// GetBudgetStatuses 는 해당 월의 예산 대비 지출 현황.
func (a *App) GetBudgetStatuses(year, month int) ([]store.BudgetStatus, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return a.st.BudgetStatuses(year, month)
}

// ---- 대시보드 통합 ----

// DashboardData 는 대시보드 한 화면에 필요한 모든 집계를 한 번에 담는다.
type DashboardData struct {
	Summary        store.MonthlySummary  `json:"summary"`
	Prev           store.MonthlySummary  `json:"prev"`
	Trend          []store.MonthPoint    `json:"trend"`
	Daily          []store.DayPoint      `json:"daily"`
	Top            []store.NamedAmount   `json:"top"`
	Cards          []store.CardStatus    `json:"cards"`
	PaymentMethods []store.PaymentMethod `json:"paymentMethods"`
	Alerts         []Alert               `json:"alerts"`
	Budgets        []store.BudgetStatus  `json:"budgets"`
}

// GetDashboard 는 대시보드용 집계를 한 호출로 모아 돌려준다(라운드트립 감소).
func (a *App) GetDashboard(year, month int) (DashboardData, error) {
	if err := a.ensure(); err != nil {
		return DashboardData{}, err
	}
	py, pm := year, month-1
	if pm == 0 {
		py, pm = year-1, 12
	}
	var d DashboardData
	var err error
	if d.Summary, err = a.st.MonthlySummary(year, month); err != nil {
		return d, err
	}
	if d.Prev, err = a.st.MonthlySummary(py, pm); err != nil {
		return d, err
	}
	if d.Trend, err = a.st.MonthlyTrend(year, month, 6); err != nil {
		return d, err
	}
	if d.Daily, err = a.st.DailyExpenses(year, month); err != nil {
		return d, err
	}
	if d.Top, err = a.st.TopMerchants(year, month, 10); err != nil {
		return d, err
	}
	if d.Cards, err = a.st.CardStatuses(time.Now()); err != nil {
		return d, err
	}
	if d.PaymentMethods, err = a.st.ListPaymentMethods(); err != nil {
		return d, err
	}
	if d.Budgets, err = a.st.BudgetStatuses(year, month); err != nil {
		return d, err
	}
	if d.Alerts, err = a.GetAlerts(); err != nil {
		return d, err
	}
	return d, nil
}

// ---- 규칙 수동 편집 ----

// SaveRule 은 규칙을 추가/수정한다. ID 가 0이면 새 규칙.
func (a *App) SaveRule(r store.Rule) error {
	if err := a.ensure(); err != nil {
		return err
	}
	if strings.TrimSpace(r.Merchant) == "" {
		return fmt.Errorf("가맹점명은 필수입니다")
	}
	if r.AmountMin < 0 || r.AmountMax < r.AmountMin {
		return fmt.Errorf("금액 구간이 올바르지 않습니다")
	}
	if r.MemberID == nil && r.CategoryID == nil {
		return fmt.Errorf("귀속자나 카테고리 중 하나는 지정해야 합니다")
	}
	if r.ID == 0 {
		_, err := a.st.AddRule(r)
		logIf("SaveRule(추가)", err)
		return err
	}
	err := a.st.UpdateRule(r)
	logIf("SaveRule(수정)", err)
	return err
}

// ---- 고정비(정기결제) ----

// RecurringItem 은 정기 거래(반복 결제) 한 건의 해당 월 발생 현황.
type RecurringItem struct {
	Label      string `json:"label"`
	Merchant   string `json:"merchant"`
	AmountMin  int64  `json:"amountMin"`
	AmountMax  int64  `json:"amountMax"`
	Months     int    `json:"months"`     // 최근 구간에서 등장한 서로 다른 월 수(정기성 근거)
	Seen       bool   `json:"seen"`       // 해당 월에 이미 발생했는지
	SeenAmount int64  `json:"seenAmount"` // 발생한 금액
	SeenDate   string `json:"seenDate"`
}

// recurringMinMonths 는 정기로 인정할 최소 등장 월 수.
const recurringMinMonths = 2

// GetRecurringStatus 는 학습된 규칙 중 최근 6개월간 2개월 이상 등장한(=실제 반복되는)
// 거래만 골라, 해당 월에 이미 발생했는지 본다. 아직 안 빠진 항목을 위로 정렬한다.
func (a *App) GetRecurringStatus(year, month int) ([]RecurringItem, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	rules, err := a.st.ListRules()
	if err != nil {
		return nil, err
	}
	// 해당 월을 마지막으로 하는 최근 6개월 거래
	end := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0).AddDate(0, 0, -1)
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -5, 0)
	target := fmt.Sprintf("%04d-%02d", year, month)
	txs, err := a.st.ListTransactions(store.TxFilter{
		From: start.Format("2006-01-02"),
		To:   end.Format("2006-01-02"),
	})
	if err != nil {
		return nil, err
	}

	unseen := []RecurringItem{}
	seen := []RecurringItem{}
	for _, r := range rules {
		months := map[string]bool{}
		it := RecurringItem{Label: r.Label, Merchant: r.Merchant, AmountMin: r.AmountMin, AmountMax: r.AmountMax}
		for _, t := range txs {
			if !classifier.RuleMatches(r, t.Merchant, t.Amount) {
				continue
			}
			if len(t.Date) >= 7 {
				months[t.Date[:7]] = true
			}
			if len(t.Date) >= 7 && t.Date[:7] == target {
				it.Seen = true
				it.SeenAmount = t.Amount
				it.SeenDate = t.Date
			}
		}
		it.Months = len(months)
		if it.Months < recurringMinMonths {
			continue // 일회성 규칙은 정기결제로 보지 않는다
		}
		if it.Seen {
			seen = append(seen, it)
		} else {
			unseen = append(unseen, it)
		}
	}
	return append(unseen, seen...), nil
}

// ---- 알림 ----

// Alert 은 대시보드 상단에 띄울 경고/안내 한 건.
type Alert struct {
	Kind   string `json:"kind"`  // budget|card|unclassified
	Level  string `json:"level"` // danger|warn|info
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

// GetAlerts 는 현재 월 기준 예산 초과/임박, 카드 실적 미달 주의, 미분류 누적 알림을 모은다.
func (a *App) GetAlerts() ([]Alert, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	now := time.Now()
	y, m := now.Year(), int(now.Month())
	out := []Alert{}

	if bs, err := a.st.BudgetStatuses(y, m); err == nil {
		for _, b := range bs {
			switch {
			case b.Over:
				out = append(out, Alert{Kind: "budget", Level: "danger",
					Title: b.Category + " 예산 초과",
					Detail: fmt.Sprintf("%s / %s (%d%%)", won(b.Spent), won(b.Amount), b.Pct)})
			case b.Pct >= 80:
				out = append(out, Alert{Kind: "budget", Level: "warn",
					Title: b.Category + " 예산 임박",
					Detail: fmt.Sprintf("%s / %s (%d%%)", won(b.Spent), won(b.Amount), b.Pct)})
			}
		}
	}

	if paces, err := a.st.CardPaces(now); err == nil {
		for _, p := range paces {
			if !p.Achieved && p.Days > 0 && p.Elapsed*100/p.Days >= 60 {
				out = append(out, Alert{Kind: "card", Level: "warn",
					Title: p.Card.Name + " 실적 미달 주의",
					Detail: fmt.Sprintf("%d/%d일차 · %s 더 써야 달성", p.Elapsed, p.Days, won(p.Target-p.Spent))})
			}
		}
	}

	if sum, err := a.st.MonthlySummary(y, m); err == nil && sum.UnclassifiedCount > 0 {
		out = append(out, Alert{Kind: "unclassified", Level: "info",
			Title: fmt.Sprintf("미분류 거래 %d건", sum.UnclassifiedCount),
			Detail: "규칙으로 일괄 분류하거나 거래내역에서 직접 분류하세요"})
	}
	return out, nil
}

// ---- CSV 내보내기 ----

type ExportResult struct {
	Path  string `json:"path"`
	Count int    `json:"count"`
}

// ExportCSV 는 필터에 맞는 거래를 CSV(UTF-8 BOM, 엑셀 호환)로 저장한다.
func (a *App) ExportCSV(filter store.TxFilter) (ExportResult, error) {
	if err := a.ensure(); err != nil {
		return ExportResult{}, err
	}
	txs, err := a.st.ListTransactions(filter)
	if err != nil {
		return ExportResult{}, err
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: "sobi_export.csv",
		Title:           "CSV 내보내기",
		Filters:         []runtime.FileFilter{{DisplayName: "CSV 파일 (*.csv)", Pattern: "*.csv"}},
	})
	if err != nil {
		return ExportResult{}, err
	}
	if path == "" { // 사용자가 취소
		return ExportResult{}, nil
	}
	if !strings.HasSuffix(strings.ToLower(path), ".csv") {
		path += ".csv"
	}

	dir := map[string]string{"income": "수입", "expense": "지출", "transfer": "이체"}
	var b strings.Builder
	b.WriteString("\uFEFF") // 엑셀에서 한글 깨짐 방지용 BOM
	b.WriteString("날짜,구분,금액,가맹점,메모,귀속자,카테고리,결제수단\n")
	for _, t := range txs {
		fields := []string{
			t.Date, dir[t.Direction], strconv.FormatInt(t.Amount, 10),
			t.Merchant, t.Memo, t.MemberName, t.CategoryName, t.PaymentMethodName,
		}
		for i, f := range fields {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(csvField(f))
		}
		b.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		logIf("ExportCSV(쓰기)", err)
		return ExportResult{}, err
	}
	return ExportResult{Path: path, Count: len(txs)}, nil
}

// csvField 는 콤마/따옴표/줄바꿈이 있으면 따옴표로 감싸고 내부 따옴표를 이스케이프한다.
func csvField(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}

// won 은 천 단위 콤마 + "원" (백엔드 알림 문구용).
func won(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteByte(s[i])
	}
	r := b.String()
	if neg {
		r = "-" + r
	}
	return r + "원"
}

// GetCumulativeCompare 는 해당 월과 전월의 일별 누적 지출 비교.
func (a *App) GetCumulativeCompare(year, month int) (store.CumCompare, error) {
	if err := a.ensure(); err != nil {
		return store.CumCompare{}, err
	}
	return a.st.CumulativeCompare(year, month, time.Now())
}

// GetWeekdayAverages 는 해당 월의 요일별 평균 지출.
func (a *App) GetWeekdayAverages(year, month int) ([]store.WeekdayAvg, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return a.st.WeekdayAverages(year, month)
}

// GetCategoryTrend 는 (year, month) 까지 최근 n개월의 카테고리별 지출 추이. level 은 main|sub.
func (a *App) GetCategoryTrend(year, month, n int, level string) (store.CategoryTrend, error) {
	if err := a.ensure(); err != nil {
		return store.CategoryTrend{}, err
	}
	return a.st.CategoryTrend(year, month, n, level)
}
