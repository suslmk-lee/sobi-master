import { FormEvent, useCallback, useEffect, useState } from "react";
import {
  AddCategory,
  AddMember,
  AddPaymentMethod,
  DeleteCategory,
  DeleteMember,
  DeletePaymentMethod,
  DeleteRule,
  ListRules,
} from "../../wailsjs/go/main/App";
import { store } from "../../wailsjs/go/models";
import { Refs, won } from "../lib";

const KIND_LABEL: Record<string, string> = {
  income: "수입",
  expense: "지출",
  transfer: "이체",
};
const PM_LABEL: Record<string, string> = { card: "카드", cash: "현금", bank: "계좌" };

function AddRow({
  placeholder,
  options,
  onAdd,
}: {
  placeholder: string;
  options?: Record<string, string>;
  onAdd: (name: string, extra: string) => Promise<void>;
}) {
  const [name, setName] = useState("");
  const [extra, setExtra] = useState(options ? Object.keys(options)[0] : "");
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;
    await onAdd(name.trim(), extra);
    setName("");
  };
  return (
    <form className="form-row" onSubmit={submit}>
      <input
        type="text"
        placeholder={placeholder}
        value={name}
        onChange={(e) => setName(e.target.value)}
      />
      {options && (
        <select value={extra} onChange={(e) => setExtra(e.target.value)}>
          {Object.entries(options).map(([k, v]) => (
            <option key={k} value={k}>{v}</option>
          ))}
        </select>
      )}
      <button type="submit">추가</button>
    </form>
  );
}

export default function SettingsPage({
  refs,
  reload,
}: {
  refs: Refs;
  reload: () => void;
}) {
  const [rules, setRules] = useState<store.Rule[]>([]);

  const loadRules = useCallback(async () => {
    setRules(await ListRules());
  }, []);

  useEffect(() => {
    loadRules();
  }, [loadRules]);

  const del = async (fn: (id: number) => Promise<void>, id: number) => {
    await fn(id);
    reload();
  };

  return (
    <div className="settings-grid">
      <div className="card">
        <h3>귀속자 (가족 구성원)</h3>
        <ul className="item-list">
          {refs.members.map((m) => (
            <li key={m.id}>
              {m.name}
              <button className="ghost" onClick={() => del(DeleteMember, m.id)}>✕</button>
            </li>
          ))}
        </ul>
        <AddRow
          placeholder="이름 (예: 할머니)"
          onAdd={async (name) => {
            await AddMember(name);
            reload();
          }}
        />
      </div>

      <div className="card">
        <h3>카테고리</h3>
        <ul className="item-list">
          {refs.categories.map((c) => (
            <li key={c.id}>
              {c.name} <span className="badge">{KIND_LABEL[c.kind]}</span>
              <button className="ghost" onClick={() => del(DeleteCategory, c.id)}>✕</button>
            </li>
          ))}
        </ul>
        <AddRow
          placeholder="카테고리명"
          options={KIND_LABEL}
          onAdd={async (name, kind) => {
            await AddCategory(name, kind);
            reload();
          }}
        />
      </div>

      <div className="card">
        <h3>결제수단 (카드/현금/계좌)</h3>
        <ul className="item-list">
          {refs.paymentMethods.map((p) => (
            <li key={p.id}>
              {p.name} <span className="badge">{PM_LABEL[p.type]}</span>
              <button className="ghost" onClick={() => del(DeletePaymentMethod, p.id)}>✕</button>
            </li>
          ))}
        </ul>
        <AddRow
          placeholder="이름 (예: 신한카드 1234)"
          options={PM_LABEL}
          onAdd={async (name, typ) => {
            await AddPaymentMethod(name, typ);
            reload();
          }}
        />
      </div>

      <div className="card rules">
        <h3>자동 분류 규칙</h3>
        <p className="muted small">
          거래를 분류할 때마다 자동으로 만들어집니다. 가맹점명과 금액 구간이 일치하면 같은
          분류를 적용합니다. 잘못 학습된 규칙은 여기서 삭제하세요.
        </p>
        <table className="tx-table">
          <thead>
            <tr>
              <th>가맹점</th>
              <th>금액 구간</th>
              <th>분류</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {rules.map((r) => (
              <tr key={r.id}>
                <td>{r.merchant}</td>
                <td className="num">{won(r.amountMin)} ~ {won(r.amountMax)}</td>
                <td>{r.label || "-"}</td>
                <td>
                  <button
                    className="ghost"
                    onClick={async () => {
                      await DeleteRule(r.id);
                      loadRules();
                    }}
                  >✕</button>
                </td>
              </tr>
            ))}
            {rules.length === 0 && (
              <tr>
                <td colSpan={4} className="muted center">아직 학습된 규칙이 없습니다</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
