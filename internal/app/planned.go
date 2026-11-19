package app

import (
	"time"

	"sobi/internal/store"
)

// ---- 예정 지출 ----
//
// 계산·저장은 store(planned.go). 자금 계획 화면의 '예정 지출' 섹션이 쓴다.

// plannedDoneDays 는 끝낸 예정 지출을 목록에 남겨 보여 주는 기간(일).
const plannedDoneDays = 60

// ListPlannedExpenses 는 아직 끝나지 않은 예정 지출과 최근 끝낸 것.
func (a *App) ListPlannedExpenses() ([]store.PlannedExpense, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	out, err := a.st.ListPlannedExpenses(time.Now(), plannedDoneDays)
	logIf("ListPlannedExpenses", err)
	return out, err
}

// SavePlannedExpense 는 예정 지출을 추가하거나 고친다.
func (a *App) SavePlannedExpense(p store.PlannedExpense) (store.PlannedExpense, error) {
	if err := a.ensure(); err != nil {
		return p, err
	}
	out, err := a.st.SavePlannedExpense(p)
	logIf("SavePlannedExpense", err)
	return out, err
}

func (a *App) DeletePlannedExpense(id int64) error {
	if err := a.ensure(); err != nil {
		return err
	}
	err := a.st.DeletePlannedExpense(id)
	logIf("DeletePlannedExpense", err)
	return err
}

// CancelPlannedExpense 는 '안 냄'으로 끝낸다(reopen 이면 다시 예정으로).
func (a *App) CancelPlannedExpense(id int64, reopen bool) error {
	if err := a.ensure(); err != nil {
		return err
	}
	err := a.st.CancelPlannedExpense(id, reopen)
	logIf("CancelPlannedExpense", err)
	return err
}

// PlannedCandidates 는 '냈어요'로 연결할 만한 지출(예정일 앞뒤 2주, 금액이 가까운 것 먼저).
func (a *App) PlannedCandidates(id int64) ([]store.Transaction, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	out, err := a.st.PlannedCandidates(id)
	logIf("PlannedCandidates", err)
	return out, err
}

// SettlePlannedExpense 는 예정 지출을 실제 거래와 잇는다(바구니에서 낼 돈이면 바구니 꺼냄도 남긴다).
func (a *App) SettlePlannedExpense(id, txID int64, moved bool) (store.PlannedExpense, error) {
	if err := a.ensure(); err != nil {
		return store.PlannedExpense{}, err
	}
	out, err := a.st.SettlePlannedExpense(id, txID, moved)
	logIf("SettlePlannedExpense", err)
	return out, err
}
