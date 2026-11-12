package main

import (
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const erSample = `{"result":"success","base_code":"KRW","time_last_update_utc":"Thu, 12 Nov 2026 00:02:31 +0000",
"rates":{"KRW":1,"USD":0.000749,"JPY":0.11916,"VND":19.19,"EUR":0.000672,"BTC":0.0000001}}`

func TestParseFxRates(t *testing.T) {
	got, err := parseFxRates(strings.NewReader(erSample))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 { // 모으는 통화만(KRW·BTC 제외)
		t.Fatalf("got %v", got)
	}
	if math.Abs(got["USD"]-1335.1135) > 0.001 || math.Abs(got["VND"]-0.05211) > 0.00001 {
		t.Fatalf("USD %v VND %v", got["USD"], got["VND"])
	}
	for _, bad := range []string{
		`{"result":"error","error-type":"unsupported-code"}`,
		`{"result":"success","base_code":"USD","rates":{"KRW":1300}}`,
		`not json`,
	} {
		if _, err := parseFxRates(strings.NewReader(bad)); err == nil {
			t.Errorf("오류여야 함: %s", bad)
		}
	}
}

// 환율은 3시간 안에는 다시 받지 않고, '새로고침'은 바로 받는다. 받은 값은 오늘 날짜로 저장된다.
func TestFetchFxRates(t *testing.T) {
	a := newTestApp(t)
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if !strings.HasPrefix(r.Header.Get("User-Agent"), "sobi/") {
			t.Errorf("User-Agent %q", r.Header.Get("User-Agent"))
		}
		w.Write([]byte(erSample))
	}))
	defer srv.Close()
	old := fxRatesURL
	fxRatesURL = srv.URL
	t.Cleanup(func() { fxRatesURL = old })
	if err := a.st.SetSetting(keyFxFetchedAt, ""); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	if err := a.fetchFxRates(a.ctx, false, now); err != nil {
		t.Fatal(err)
	}
	if err := a.fetchFxRates(a.ctx, false, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Fatalf("한 시간 뒤에 또 받음: %d", hits)
	}
	if err := a.fetchFxRates(a.ctx, true, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if hits != 2 {
		t.Fatalf("새로고침이 안 받음: %d", hits)
	}
	ov, err := a.GetFxOverview()
	if err != nil {
		t.Fatal(err)
	}
	if ov.RateDate != now.Format("2006-01-02") || ov.RateSource == "" || ov.FetchError != "" {
		t.Fatalf("overview date=%s src=%s err=%s", ov.RateDate, ov.RateSource, ov.FetchError)
	}

	// 받는 곳이 망가지면 새로고침은 오류를 알려 주고, 화면은 있던 환율로 보여 준다
	fxRatesURL = srv.URL + "/missing"
	srv.Config.Handler = http.NotFoundHandler()
	ov, err = a.RefreshFxRates()
	if err != nil || ov.FetchError == "" || ov.RateDate == "" {
		t.Fatalf("실패 표시: err=%v fetchErr=%q date=%s", err, ov.FetchError, ov.RateDate)
	}
}
