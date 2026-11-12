package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"sobi/internal/store"
)

// ---- 외화 모으기 ----
//
// 장부·자동 기록 계산은 store(fx.go). 여기서는 환율을 받아 오고, 시간마다 계획을 돌린다.
//
// 환율은 ExchangeRate-API 의 무료 공개 주소(키 없음)에서 하루 한 번 갱신되는 값을 받는다.
// 토스뱅크는 매매기준율로 사고팔아 몇 원 차이가 날 수 있다 — 자동 기록은 '추정'으로 남기고,
// 실제 체결 환율로 고칠 수 있게 한다. 요청에는 통화 기준(KRW) 말고 아무 정보도 싣지 않는다.

// fxRatesURL 은 환율을 받는 주소(테스트에서 바꿔 끼운다).
var fxRatesURL = "https://open.er-api.com/v6/latest/KRW"

const (
	fxRateSource = "ExchangeRate-API"
	// fxFetchEvery 는 환율을 다시 받아 오는 간격. 원본이 하루 한 번 바뀌므로 자주 받을 필요가 없다
	// (제공처도 한 시간에 한 번 넘게 부르지 말라고 한다).
	fxFetchEvery = 3 * time.Hour
	// keyFxFetchedAt 은 마지막으로 환율을 받은 시각(RFC3339)을 두는 app_settings 키.
	keyFxFetchedAt = "fx.fetched_at"
)

// parseFxRates 는 ExchangeRate-API 응답(1원당 외화)을 외화 1단위당 원화로 바꾼다. 모으는 통화만.
func parseFxRates(r io.Reader) (map[string]float64, error) {
	var body struct {
		Result string             `json:"result"`
		Base   string             `json:"base_code"`
		Rates  map[string]float64 `json:"rates"`
	}
	if err := json.NewDecoder(io.LimitReader(r, 1<<20)).Decode(&body); err != nil {
		return nil, fmt.Errorf("환율 응답을 읽지 못했습니다: %w", err)
	}
	if body.Result != "success" || body.Base != "KRW" {
		return nil, fmt.Errorf("환율을 받지 못했습니다(%s)", body.Result)
	}
	out := map[string]float64{}
	for _, c := range store.FxCurrencies {
		if v := body.Rates[c.Code]; v > 0 {
			out[c.Code] = 1 / v
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("환율 응답에 모으는 통화가 없습니다")
	}
	return out, nil
}

// fetchFxRates 는 환율을 받아 오늘 날짜로 저장한다. force 가 아니면 fxFetchEvery 안에는 다시 받지 않는다.
func (a *App) fetchFxRates(ctx context.Context, force bool, now time.Time) error {
	if !force {
		if last, err := a.st.Setting(keyFxFetchedAt); err == nil && last != "" {
			if t, err := time.Parse(time.RFC3339, last); err == nil && now.Sub(t) < fxFetchEvery {
				return nil
			}
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fxRatesURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "sobi/"+appVersion)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("환율을 받지 못했습니다: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("환율을 받지 못했습니다(HTTP %d)", resp.StatusCode)
	}
	rates, err := parseFxRates(resp.Body)
	if err != nil {
		return err
	}
	if err := a.st.SaveFxRates(now.Format("2006-01-02"), rates, fxRateSource); err != nil {
		return err
	}
	return a.st.SetSetting(keyFxFetchedAt, now.Format(time.RFC3339))
}

// runFx 는 환율을 (필요하면) 받고 외화 모으기 계획을 오늘까지 돌린다. 환율을 못 받아도 있던 값으로 돈다.
func (a *App) runFx(force bool) error {
	now := time.Now()
	fetchErr := a.fetchFxRates(a.ctx, force, now)
	logIf("외화 환율 받기", fetchErr)
	n, err := a.st.RunFxPlans(now)
	if err != nil {
		return err
	}
	if n > 0 {
		log.Printf("외화 모으기 자동 기록 %d건", n)
	}
	if force {
		return fetchErr
	}
	return nil
}

// fxScreen 은 외화 화면 데이터에 환율 출처와 이번에 환율을 받다 생긴 문제(화면에 한 줄로 보여 준다)를 붙인다.
func (a *App) fxScreen(fetchErr error) (store.FxOverview, error) {
	out, err := a.st.GetFxOverview()
	out.RateSource = fxRateSource
	if fetchErr != nil {
		out.FetchError = fetchErr.Error()
	}
	return out, err
}

// GetFxOverview 는 외화 화면: 보유 외화·평가손익·환율·계획·거래. 환율이 오래됐으면 받아 오고 계획을 돌린다.
func (a *App) GetFxOverview() (store.FxOverview, error) {
	if err := a.ensure(); err != nil {
		return store.FxOverview{}, err
	}
	now := time.Now()
	fetchErr := a.fetchFxRates(a.ctx, false, now)
	logIf("외화 환율 받기", fetchErr)
	if _, err := a.st.RunFxPlans(now); err != nil {
		logIf("외화 모으기 자동 기록", err)
	}
	out, err := a.fxScreen(fetchErr)
	logIf("GetFxOverview", err)
	return out, err
}

// RefreshFxRates 는 환율을 지금 다시 받아 오고 계획을 돌린다.
func (a *App) RefreshFxRates() (store.FxOverview, error) {
	if err := a.ensure(); err != nil {
		return store.FxOverview{}, err
	}
	fetchErr := a.runFx(true)
	return a.fxScreen(fetchErr)
}

// SaveFxTrade 는 외화 사기·팔기(처음 잔고 포함)를 추가하거나 고친다.
func (a *App) SaveFxTrade(t store.FxTrade) (store.FxTrade, error) {
	if err := a.ensure(); err != nil {
		return t, err
	}
	out, err := a.st.SaveFxTrade(t)
	logIf("SaveFxTrade", err)
	return out, err
}

func (a *App) DeleteFxTrade(id int64) error {
	if err := a.ensure(); err != nil {
		return err
	}
	err := a.st.DeleteFxTrade(id)
	logIf("DeleteFxTrade", err)
	return err
}

// SaveFxPlan 은 매일 모으기·목표 환율 매수 계획을 추가하거나 고친다. 저장하고 바로 오늘 몫을 돌린다.
func (a *App) SaveFxPlan(p store.FxPlan) (store.FxPlan, error) {
	if err := a.ensure(); err != nil {
		return p, err
	}
	out, err := a.st.SaveFxPlan(p)
	logIf("SaveFxPlan", err)
	if err == nil {
		logIf("외화 모으기(계획 저장 후)", a.runFx(false))
	}
	return out, err
}

func (a *App) DeleteFxPlan(id int64) error {
	if err := a.ensure(); err != nil {
		return err
	}
	err := a.st.DeleteFxPlan(id)
	logIf("DeleteFxPlan", err)
	return err
}
