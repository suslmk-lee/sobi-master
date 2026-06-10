// Package importer 는 카드사/은행에서 내려받은 CSV 이용내역을 거래로 변환한다.
// 헤더의 키워드(이용일/가맹점/금액/입금/출금 등)로 컬럼을 자동 인식하고,
// EUC-KR 로 인코딩된 파일도 UTF-8 로 변환해 처리한다.
package importer

import (
	"encoding/csv"
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/transform"

	"sobi/internal/store"
)

// ParsedRow 는 CSV 한 줄을 해석한 결과.
type ParsedRow struct {
	Date      string `json:"date"`
	Amount    int64  `json:"amount"`
	Direction string `json:"direction"`
	Merchant  string `json:"merchant"`
	Memo      string `json:"memo"`
}

type Result struct {
	Total          int      `json:"total"`
	Parsed         []ParsedRow `json:"parsed"`
	Errors         []string `json:"errors"`
}

// ParseFile 은 CSV 파일을 읽어 거래 행으로 변환한다. DB에는 넣지 않는다.
func ParseFile(path string) (Result, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Result{}, err
	}
	return Parse(raw)
}

func Parse(raw []byte) (Result, error) {
	text := decode(raw)
	r := csv.NewReader(strings.NewReader(text))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	records, err := r.ReadAll()
	if err != nil {
		return Result{}, fmt.Errorf("CSV 파싱 실패: %w", err)
	}

	res := Result{Parsed: []ParsedRow{}, Errors: []string{}}
	if len(records) == 0 {
		return res, fmt.Errorf("빈 파일입니다")
	}

	// 헤더 행 탐색: 처음 10행 안에서 날짜+금액 계열 컬럼이 모두 보이는 행
	headerIdx := -1
	var cols colMap
	for i := 0; i < len(records) && i < 10; i++ {
		if c, ok := mapColumns(records[i]); ok {
			headerIdx, cols = i, c
			break
		}
	}
	if headerIdx < 0 {
		return res, fmt.Errorf("날짜/금액 컬럼을 찾지 못했습니다. CSV 첫 행에 헤더(이용일, 가맹점명, 이용금액 등)가 있는지 확인하세요")
	}

	for i := headerIdx + 1; i < len(records); i++ {
		row := records[i]
		pr, err := cols.parseRow(row)
		if err != nil {
			if !isBlankRow(row) {
				res.Errors = append(res.Errors, fmt.Sprintf("%d행: %v", i+1, err))
			}
			continue
		}
		res.Parsed = append(res.Parsed, pr)
	}
	res.Total = len(res.Parsed)
	return res, nil
}

// decode 는 EUC-KR(CP949) 파일을 UTF-8 로 변환한다. 이미 UTF-8 이면 그대로 둔다.
func decode(raw []byte) string {
	if utf8.Valid(raw) {
		return strings.TrimPrefix(string(raw), "\uFEFF")
	}
	out, _, err := transform.Bytes(korean.EUCKR.NewDecoder(), raw)
	if err != nil {
		return string(raw)
	}
	return string(out)
}

type colMap struct {
	date     int
	merchant int
	memo     int
	amount   int // 단일 금액 컬럼 (카드 내역)
	deposit  int // 입금 (은행 내역)
	withdraw int // 출금 (은행 내역)
}

var (
	dateKeys     = []string{"이용일", "거래일", "승인일", "결제일", "날짜", "일자", "date"}
	merchantKeys = []string{"가맹점", "이용하신곳", "거래처", "적요", "내용", "이용내역", "merchant", "상호"}
	amountKeys   = []string{"이용금액", "승인금액", "결제금액", "거래금액", "금액", "amount"}
	depositKeys  = []string{"입금", "받은금액", "deposit"}
	withdrawKeys = []string{"출금", "보낸금액", "withdraw"}
	memoKeys     = []string{"메모", "비고", "memo"}
)

func mapColumns(header []string) (colMap, bool) {
	c := colMap{date: -1, merchant: -1, memo: -1, amount: -1, deposit: -1, withdraw: -1}
	find := func(keys []string, cell string) bool {
		cell = strings.ToLower(strings.TrimSpace(cell))
		for _, k := range keys {
			if strings.Contains(cell, k) {
				return true
			}
		}
		return false
	}
	for i, cell := range header {
		switch {
		case c.date < 0 && find(dateKeys, cell):
			c.date = i
		case c.withdraw < 0 && find(withdrawKeys, cell):
			c.withdraw = i
		case c.deposit < 0 && find(depositKeys, cell):
			c.deposit = i
		case c.amount < 0 && find(amountKeys, cell):
			c.amount = i
		case c.merchant < 0 && find(merchantKeys, cell):
			c.merchant = i
		case c.memo < 0 && find(memoKeys, cell):
			c.memo = i
		}
	}
	ok := c.date >= 0 && (c.amount >= 0 || c.deposit >= 0 || c.withdraw >= 0)
	return c, ok
}

func (c colMap) parseRow(row []string) (ParsedRow, error) {
	get := func(i int) string {
		if i < 0 || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}
	pr := ParsedRow{Merchant: get(c.merchant), Memo: get(c.memo)}

	date, err := normalizeDate(get(c.date))
	if err != nil {
		return pr, err
	}
	pr.Date = date

	// 은행 내역(입금/출금 분리) 우선, 아니면 단일 금액 컬럼
	if dep := parseAmount(get(c.deposit)); dep > 0 {
		pr.Amount, pr.Direction = dep, "income"
	} else if wd := parseAmount(get(c.withdraw)); wd > 0 {
		pr.Amount, pr.Direction = wd, "expense"
	} else if amt := parseAmount(get(c.amount)); amt != 0 {
		pr.Direction = "expense"
		if amt < 0 { // 음수면 취소/환불 → 수입으로
			amt = -amt
			pr.Direction = "income"
		}
		pr.Amount = amt
	} else {
		return pr, fmt.Errorf("금액 없음")
	}
	return pr, nil
}

var dateRe = regexp.MustCompile(`(\d{4})[.\-/년\s]*(\d{1,2})[.\-/월\s]*(\d{1,2})`)

func normalizeDate(s string) (string, error) {
	m := dateRe.FindStringSubmatch(s)
	if m == nil {
		return "", fmt.Errorf("날짜 형식을 인식할 수 없음: %q", s)
	}
	return fmt.Sprintf("%s-%02s-%02s", m[1], pad(m[2]), pad(m[3])), nil
}

func pad(s string) string {
	if len(s) == 1 {
		return "0" + s
	}
	return s
}

var amountClean = regexp.MustCompile(`[^\d\-]`)

func parseAmount(s string) int64 {
	if s == "" {
		return 0
	}
	cleaned := amountClean.ReplaceAllString(s, "")
	if cleaned == "" || cleaned == "-" {
		return 0
	}
	var v int64
	fmt.Sscanf(cleaned, "%d", &v)
	return v
}

func isBlankRow(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

// ToTransaction 은 파싱된 행을 거래 모델로 바꾼다.
func (p ParsedRow) ToTransaction() store.Transaction {
	return store.Transaction{
		Date:      p.Date,
		Amount:    p.Amount,
		Direction: p.Direction,
		Merchant:  p.Merchant,
		Memo:      p.Memo,
		Source:    "import",
	}
}
