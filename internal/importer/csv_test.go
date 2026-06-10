package importer

import (
	"testing"

	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/transform"
)

func TestParseCardCSV(t *testing.T) {
	csv := `이용일자,가맹점명,이용금액,비고
2026.05.15,LG유플러스,"65,890",자동결제
2026.05.15,LG유플러스,"38,500",자동결제
2026-05-20,스타벅스 강남점,"6,100",
`
	res, err := Parse([]byte(csv))
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 3 {
		t.Fatalf("총 %d건, want 3 (errors: %v)", res.Total, res.Errors)
	}
	p := res.Parsed[0]
	if p.Date != "2026-05-15" || p.Amount != 65890 || p.Merchant != "LG유플러스" || p.Direction != "expense" {
		t.Errorf("첫 행 파싱 오류: %+v", p)
	}
	if res.Parsed[2].Date != "2026-05-20" || res.Parsed[2].Amount != 6100 {
		t.Errorf("날짜/금액 형식 파싱 오류: %+v", res.Parsed[2])
	}
}

func TestParseBankCSV(t *testing.T) {
	csv := `거래일시,적요,출금액,입금액,잔액
2026/05/25,급여 (주)퀀텀,,"4,200,000","5,000,000"
2026/05/26,주택담보대출 상환,"1,150,000",,"3,850,000"
2026/05/27,키움증권,"500,000",,"3,350,000"
`
	res, err := Parse([]byte(csv))
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 3 {
		t.Fatalf("총 %d건, want 3 (errors: %v)", res.Total, res.Errors)
	}
	if res.Parsed[0].Direction != "income" || res.Parsed[0].Amount != 4200000 {
		t.Errorf("급여 입금 파싱 오류: %+v", res.Parsed[0])
	}
	if res.Parsed[1].Direction != "expense" || res.Parsed[1].Amount != 1150000 {
		t.Errorf("대출 출금 파싱 오류: %+v", res.Parsed[1])
	}
}

// 카드사 CSV가 EUC-KR 로 인코딩된 경우에도 한글이 깨지지 않아야 한다.
func TestParseEUCKR(t *testing.T) {
	utf8CSV := "이용일,가맹점명,이용금액\n2026.05.15,엘지유플러스,65890\n"
	euckr, _, err := transform.Bytes(korean.EUCKR.NewEncoder(), []byte(utf8CSV))
	if err != nil {
		t.Fatal(err)
	}
	res, err := Parse(euckr)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 || res.Parsed[0].Merchant != "엘지유플러스" {
		t.Fatalf("EUC-KR 파싱 오류: %+v (errors: %v)", res.Parsed, res.Errors)
	}
}
