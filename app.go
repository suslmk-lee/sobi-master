package main

import (
	"context"
	"fmt"
	"log"
	"os"
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
}

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
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.st != nil {
		a.st.Close()
	}
}

func logIf(op string, err error) {
	if err != nil {
		log.Printf("%s: %v", op, err)
	}
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

func (a *App) AddCategory(name, kind string) (store.Category, error) {
	if err := a.ensure(); err != nil {
		return store.Category{}, err
	}
	c, err := a.st.AddCategory(name, kind)
	logIf("AddCategory", err)
	return c, err
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

func (a *App) ListTransactions(month string, unclassifiedOnly bool) ([]store.Transaction, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return a.st.ListTransactions(month, unclassifiedOnly)
}

// AddTransaction 은 수동 등록. 귀속자/카테고리가 비어 있으면 규칙으로 자동 분류를
// 시도하고, 사용자가 직접 분류해서 넣었으면 그 분류를 규칙으로 학습한다.
func (a *App) AddTransaction(t store.Transaction) (int64, error) {
	if err := a.ensure(); err != nil {
		return 0, err
	}
	if t.Date == "" || t.Amount <= 0 {
		return 0, fmt.Errorf("날짜와 금액(양수)은 필수입니다")
	}
	t.Source = "manual"
	userClassified := t.MemberID != nil || t.CategoryID != nil
	if !userClassified {
		rule, err := a.cl.Match(t.Merchant, t.Amount)
		if err != nil {
			logIf("AddTransaction(분류)", err)
			return 0, err
		}
		classifier.Apply(&t, rule)
	}
	id, err := a.st.AddTransaction(t)
	if err != nil {
		logIf("AddTransaction", err)
		return 0, err
	}
	if userClassified {
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
	return a.st.DeleteTransaction(id)
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
