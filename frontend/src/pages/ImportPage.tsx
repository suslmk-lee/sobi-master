import { useState } from "react";
import { ImportCSV } from "../../wailsjs/go/main/App";
import { main } from "../../wailsjs/go/models";
import { Refs } from "../lib";

export default function ImportPage({ refs }: { refs: Refs }) {
  const [pmId, setPmId] = useState("");
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<main.ImportResult | null>(null);
  const [err, setErr] = useState("");

  const run = async () => {
    setBusy(true);
    setErr("");
    try {
      const res = await ImportCSV(pmId ? Number(pmId) : 0);
      if (res.file) {
        setResult(res);
      }
    } catch (e: any) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div>
      <div className="card">
        <h3>카드/은행 이용내역 가져오기 (CSV)</h3>
        <p className="muted">
          카드사·은행 홈페이지에서 내려받은 CSV 내역 파일을 선택하세요. 이용일/가맹점/금액
          컬럼을 자동으로 인식하고, 이미 등록된 거래(같은 날짜·금액·가맹점)는 건너뜁니다.
          이전에 분류해 둔 반복 거래는 자동으로 귀속자·카테고리가 붙습니다.
        </p>
        <div className="form-row">
          <select value={pmId} onChange={(e) => setPmId(e.target.value)}>
            <option value="">결제수단(어느 카드/계좌) 선택 — 선택 안 함</option>
            {refs.paymentMethods.map((p) => (
              <option key={p.id} value={p.id}>{p.name}</option>
            ))}
          </select>
          <button onClick={run} disabled={busy}>
            {busy ? "가져오는 중…" : "CSV 파일 선택"}
          </button>
        </div>
        {err && <p className="error">{err}</p>}
      </div>

      {result && (
        <div className="card">
          <h3>가져오기 결과</h3>
          <p className="muted small">{result.file}</p>
          <ul className="result-list">
            <li>인식된 거래: <strong>{result.total}건</strong></li>
            <li>새로 등록: <strong>{result.imported}건</strong></li>
            <li>자동 분류: <strong>{result.autoClassified}건</strong></li>
            <li>중복 건너뜀: <strong>{result.duplicates}건</strong></li>
          </ul>
          {result.errors && result.errors.length > 0 && (
            <details>
              <summary className="error">처리 못한 행 {result.errors.length}개</summary>
              <ul>
                {result.errors.map((e, i) => (
                  <li key={i} className="small">{e}</li>
                ))}
              </ul>
            </details>
          )}
          {result.imported - result.autoClassified > 0 && (
            <p>
              아직 분류 안 된 거래가 있습니다. <strong>거래내역 탭에서 "미분류만
              보기"</strong>를 켜고 귀속자·카테고리를 지정하세요. 한 번 지정하면 다음 달부터
              자동으로 분류됩니다.
            </p>
          )}
        </div>
      )}
    </div>
  );
}
