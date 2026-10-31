import {
  DragEvent as ReactDragEvent,
  FormEvent,
  useCallback,
  useEffect,
  useState,
} from "react";
import {
  AddCategory,
  AddMember,
  AddPaymentMethod,
  BackupNow,
  DeleteBudget,
  DeleteCategory,
  DeleteMember,
  DeletePaymentMethod,
  DeleteRule,
  MoveCategory,
  MoveCategoryTo,
  GetLastBackup,
  ListBudgets,
  ListRules,
  SaveRule,
  SetBudget,
  SetCategoryParent,
} from "../../wailsjs/go/main/App";
import { CornerDownRight, GripVertical, Palette, Pencil, Plus, Trash2, X } from "lucide-react";
import { main, store } from "../../wailsjs/go/models";
import AppearanceSettings from "../components/settings/AppearanceSettings";
import {
  categoryOptions,
  formatAmount,
  mainCategories,
  parseAmount,
  Refs,
  thisMonth,
  won,
} from "../lib";
import { useConfirm } from "../ConfirmModal";
import HolidaySection from "../HolidaySection";
import MonthPicker from "../MonthPicker";

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
    <form className="flex flex-wrap items-center gap-2" onSubmit={submit}>
      <input
        type="text"
        className="input-field !w-auto min-w-[10rem] flex-1"
        placeholder={placeholder}
        value={name}
        onChange={(e) => setName(e.target.value)}
      />
      {options && (
        <select className="input-field !w-auto" value={extra} onChange={(e) => setExtra(e.target.value)}>
          {Object.entries(options).map(([k, v]) => (
            <option key={k} value={k}>{v}</option>
          ))}
        </select>
      )}
      <button type="submit" className="btn-primary">추가</button>
    </form>
  );
}

// 섹션 제목 + 설명
function SectionHead({ title, desc }: { title: string; desc: string }) {
  return (
    <div className="space-y-1">
      <h3 className="section-title">{title}</h3>
      <p className="text-xs leading-relaxed text-zinc-400">{desc}</p>
    </div>
  );
}

// 카테고리 예산: "매월 기본"('*') 또는 "특정 월"(YYYY-MM) 예산을 정한다. 0 저장 시 해제.
function BudgetSection({ refs }: { refs: Refs }) {
  const [scope, setScope] = useState<"default" | "month">("default");
  const [ym, setYm] = useState(thisMonth());
  const [vals, setVals] = useState<Record<number, string>>({});
  const [saved, setSaved] = useState<Record<number, number>>({});
  const expenseCats = refs.categories.filter((c) => c.kind === "expense");

  const effectiveYm = scope === "default" ? "*" : ym;

  const load = useCallback(async () => {
    const list = await ListBudgets(effectiveYm);
    const m: Record<number, number> = {};
    const f: Record<number, string> = {};
    for (const b of list) {
      m[b.categoryId] = b.amount;
      f[b.categoryId] = formatAmount(String(b.amount));
    }
    setSaved(m);
    setVals(f);
  }, [effectiveYm]);
  useEffect(() => {
    load();
  }, [load]);

  const save = async (catId: number) => {
    await SetBudget(catId, effectiveYm, parseAmount(vals[catId] || ""));
    load();
  };
  const clear = async (catId: number) => {
    await DeleteBudget(catId, effectiveYm);
    setVals((p) => ({ ...p, [catId]: "" }));
    load();
  };
  const total = Object.values(saved).reduce((a, b) => a + b, 0);

  return (
    <div className="surface space-y-4 p-5">
      <h3 className="section-title">카테고리 예산</h3>
      <div className="flex flex-wrap items-center gap-3">
        <div className="glass-pill inline-flex p-1">
          <button
            className={`pill-item ${scope === "default" ? "pill-item-active" : ""}`}
            onClick={() => setScope("default")}
          >매월 기본</button>
          <button
            className={`pill-item ${scope === "month" ? "pill-item-active" : ""}`}
            onClick={() => setScope("month")}
          >특정 월</button>
        </div>
        {scope === "month" && (
          <MonthPicker value={ym} onChange={setYm} />
        )}
      </div>
      <p className="text-xs text-zinc-400">
        {scope === "default"
          ? "지출 카테고리별 매월 기본 예산입니다. 대시보드에서 사용률·초과를 추적합니다."
          : "해당 월에만 적용할 예산입니다. 비워서 저장하면 그 달은 매월 기본 예산을 따릅니다."}
        {" "}주에 예산을 걸면 하위 부 지출까지 합산해 판정합니다.
      </p>
      <div className="space-y-1.5">
        {expenseCats.map((c) => (
          <div className="flex items-center gap-2" key={c.id}>
            <span
              className={`min-w-0 flex-1 truncate text-sm ${c.parentId ? "pl-4 text-zinc-400" : "text-zinc-100"}`}
              title={c.fullName}
            >
              {c.parentId ? `└ ${c.name}` : c.name}
            </span>
            <input
              type="text"
              className="input-field num !w-40 text-right"
              inputMode="numeric"
              placeholder={scope === "month" ? "기본 따름" : "예산 없음"}
              value={vals[c.id] || ""}
              onChange={(e) =>
                setVals((p) => ({ ...p, [c.id]: formatAmount(e.target.value) }))
              }
              onKeyDown={(e) => e.key === "Enter" && save(c.id)}
            />
            <button className="btn-ghost !px-3 !py-2 !text-xs" onClick={() => save(c.id)}>저장</button>
            <span className="inline-flex h-8 w-8 shrink-0 items-center justify-center">
              {saved[c.id] > 0 && (
                <button className="btn-icon hover:!text-rose-300" onClick={() => clear(c.id)} title="예산 해제" aria-label="예산 해제">
                  <X className="h-4 w-4" />
                </button>
              )}
            </span>
          </div>
        ))}
      </div>
      <p className="label-mono">
        {scope === "default" ? "설정된 총 기본 예산" : `${Number(ym.slice(5))}월 전용 예산 합계`}{" "}
        <span className="num text-white">{won(total)}</span>
      </p>
    </div>
  );
}

const RULE_BLANK = {
  id: 0,
  merchant: "",
  amountMin: "",
  amountMax: "",
  memberId: "",
  categoryId: "",
};
type RuleForm = typeof RULE_BLANK;

// 자동 분류 규칙: 학습된 규칙을 보고 직접 추가/수정/삭제한다.
function RuleSection({ refs }: { refs: Refs }) {
  const [rules, setRules] = useState<store.Rule[]>([]);
  const [f, setF] = useState<RuleForm>({ ...RULE_BLANK });
  const [err, setErr] = useState("");
  const [confirm, confirmUI] = useConfirm();

  const load = useCallback(async () => setRules(await ListRules()), []);
  useEffect(() => {
    load();
  }, [load]);

  const nameOf = (list: { id: number; name: string }[], id?: number) =>
    list.find((x) => x.id === id)?.name || "";

  const edit = (r: store.Rule) =>
    setF({
      id: r.id,
      merchant: r.merchant,
      amountMin: formatAmount(String(r.amountMin)),
      amountMax: formatAmount(String(r.amountMax)),
      memberId: r.memberId ? String(r.memberId) : "",
      categoryId: r.categoryId ? String(r.categoryId) : "",
    });

  const save = async (e: FormEvent) => {
    e.preventDefault();
    setErr("");
    const min = parseAmount(f.amountMin);
    const max = parseAmount(f.amountMax);
    if (!f.merchant.trim()) return setErr("가맹점명을 입력하세요.");
    if (max < min) return setErr("최대 금액이 최소보다 작습니다.");
    const memberName = nameOf(refs.members, Number(f.memberId));
    const categoryName = nameOf(refs.categories, Number(f.categoryId));
    const label = [memberName, categoryName].filter(Boolean).join(" · ");
    try {
      await SaveRule(
        new store.Rule({
          id: f.id,
          merchant: f.merchant.trim(),
          amountMin: min,
          amountMax: max,
          memberId: f.memberId ? Number(f.memberId) : undefined,
          categoryId: f.categoryId ? Number(f.categoryId) : undefined,
          label,
        })
      );
      setF({ ...RULE_BLANK });
      load();
    } catch (e: any) {
      setErr(String(e));
    }
  };

  return (
    <div className="surface space-y-4 p-5">
      {confirmUI}
      <SectionHead
        title="자동 분류 규칙"
        desc="거래를 분류할 때마다 자동으로 만들어집니다. 직접 추가·수정할 수도 있습니다. 가맹점명과 금액 구간이 일치하면 같은 분류를 적용합니다."
      />
      <form className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3" onSubmit={save}>
        <input
          type="text"
          className="input-field"
          placeholder="가맹점 (예: 스타벅스)"
          value={f.merchant}
          onChange={(e) => setF({ ...f, merchant: e.target.value })}
        />
        <input
          type="text"
          className="input-field num"
          inputMode="numeric"
          placeholder="최소 금액"
          value={f.amountMin}
          onChange={(e) => setF({ ...f, amountMin: formatAmount(e.target.value) })}
        />
        <input
          type="text"
          className="input-field num"
          inputMode="numeric"
          placeholder="최대 금액"
          value={f.amountMax}
          onChange={(e) => setF({ ...f, amountMax: formatAmount(e.target.value) })}
        />
        <select className="input-field" value={f.memberId} onChange={(e) => setF({ ...f, memberId: e.target.value })}>
          <option value="">귀속자</option>
          {refs.members.map((m) => (
            <option key={m.id} value={m.id}>{m.name}</option>
          ))}
        </select>
        <select className="input-field" value={f.categoryId} onChange={(e) => setF({ ...f, categoryId: e.target.value })}>
          <option value="">카테고리</option>
          {categoryOptions(refs.categories).map((c) => (
            <option key={c.id} value={c.id}>{c.label}</option>
          ))}
        </select>
        <div className="flex gap-2">
          <button type="submit" className="btn-primary flex-1">{f.id ? "수정" : "추가"}</button>
          {f.id !== 0 && (
            <button type="button" className="btn-ghost" onClick={() => setF({ ...RULE_BLANK })}>취소</button>
          )}
        </div>
      </form>
      {err && <p className="text-sm text-rose-300">{err}</p>}
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="table-head border-b border-white/[0.07] text-left">
              <th className="py-2 pr-3 font-normal">가맹점</th>
              <th className="py-2 pr-3 text-right font-normal">금액 구간</th>
              <th className="py-2 pr-3 font-normal">분류</th>
              <th className="py-2 font-normal"></th>
            </tr>
          </thead>
          <tbody>
            {rules.map((r) => (
              <tr key={r.id} className={`table-row ${f.id === r.id ? "bg-mint-400/10" : ""}`}>
                <td className="py-2.5 pr-3 text-zinc-100">{r.merchant}</td>
                <td className="num py-2.5 pr-3 text-right">{won(r.amountMin)} ~ {won(r.amountMax)}</td>
                <td className="py-2.5 pr-3 text-zinc-300">{r.label || "-"}</td>
                <td className="py-2.5">
                  <div className="flex justify-end gap-1">
                    <button className="btn-icon" onClick={() => edit(r)} title="수정" aria-label="수정">
                      <Pencil className="h-4 w-4" />
                    </button>
                    <button
                      className="btn-icon hover:!text-rose-300"
                      onClick={async () => {
                        const ok = await confirm({
                          title: `"${r.merchant}" 규칙을 삭제할까요?`,
                          message: "이미 분류된 거래는 바뀌지 않습니다. 다음 분류부터 이 규칙이 쓰이지 않습니다.",
                          danger: true,
                          confirmLabel: "삭제",
                        });
                        if (!ok) return;
                        await DeleteRule(r.id);
                        load();
                      }}
                      title="삭제"
                      aria-label="삭제"
                    ><Trash2 className="h-4 w-4" /></button>
                  </div>
                </td>
              </tr>
            ))}
            {rules.length === 0 && (
              <tr>
                <td colSpan={4} className="py-6 text-center text-zinc-500">아직 규칙이 없습니다</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// 카테고리: 주(대분류)/부(소분류) 2단 계층을 종류(지출/수입/이체)별 트리로 보여준다.
// 부의 종류는 주를 따르고, 통계·예산은 기본적으로 주 기준으로 합산된다.
function CategorySection({ refs, reload }: { refs: Refs; reload: () => void }) {
  const [err, setErr] = useState("");
  const [confirm, confirmUI] = useConfirm();
  // 어느 주 아래에 부를 추가하는 중인지
  const [addingUnder, setAddingUnder] = useState<number | null>(null);
  const [subName, setSubName] = useState("");
  // 새 주 카테고리
  const [mainName, setMainName] = useState("");
  const [mainKind, setMainKind] = useState("expense");

  const run = async (fn: () => Promise<unknown>) => {
    setErr("");
    try {
      await fn();
      reload();
    } catch (e: any) {
      setErr(String(e));
    }
  };

  const addMain = async (e: FormEvent) => {
    e.preventDefault();
    if (!mainName.trim()) return;
    await run(async () => {
      await AddCategory(mainName.trim(), mainKind, 0);
      setMainName("");
    });
  };

  const addSub = async (e: FormEvent, parentId: number) => {
    e.preventDefault();
    if (!subName.trim()) return;
    await run(async () => {
      await AddCategory(subName.trim(), "expense", parentId); // 종류는 주를 따른다
      setSubName("");
      setAddingUnder(null);
    });
  };

  // 삭제 시 영향을 알린다: 이 카테고리를 쓰던 거래는 미분류가 되고, 부는 주로 올라간다.
  const remove = async (name: string, id: number, subCount: number) => {
    const ok = await confirm({
      title: `"${name}" 카테고리를 삭제할까요?`,
      danger: true,
      confirmLabel: "삭제",
      message: (
        <>
          이 카테고리로 분류된 거래는 <strong>미분류</strong>가 됩니다.
          {subCount > 0 && <> 부 카테고리 {subCount}개는 주 카테고리로 올라갑니다.</>}
        </>
      ),
    });
    if (!ok) return;
    run(() => DeleteCategory(id));
  };

  // 종류별 → 주별로 부를 묶은 트리 (순서는 백엔드가 정한 표시 순서 그대로)
  const treeOf = (kind: string) =>
    mainCategories(refs.categories, kind).map((m) => ({
      main: m,
      subs: refs.categories.filter((c) => c.parentId === m.id),
    }));

  // ---- 표시 순서 드래그 ----
  // 같은 그룹 안에서만 옮길 수 있다. group 키가 같은 행끼리만 서로 드롭을 받는다.
  // (주는 "같은 종류의 주끼리", 부는 "같은 주 아래 부끼리")
  const [drag, setDrag] = useState<{ id: number; group: string } | null>(null);
  const [overId, setOverId] = useState<number | null>(null);

  const canDropOn = (group: string, id: number) =>
    !!drag && drag.group === group && drag.id !== id;

  // 드래그 손잡이 + 그 행을 드롭 대상으로 만드는 속성 묶음.
  // 손잡이만 draggable 로 둬서 행 안의 셀렉트·버튼 조작을 방해하지 않는다.
  const dragProps = (id: number, group: string, index: number) => ({
    onDragOver: (e: ReactDragEvent) => {
      if (!canDropOn(group, id)) return;
      e.preventDefault(); // 이걸 해야 드롭이 허용된다
      setOverId(id);
    },
    onDragLeave: () => setOverId((prev) => (prev === id ? null : prev)),
    onDrop: (e: ReactDragEvent) => {
      e.preventDefault();
      const d = drag;
      setOverId(null);
      setDrag(null);
      if (!d || !canDropOn(group, id)) return;
      run(() => MoveCategoryTo(d.id, index));
    },
  });

  const grip = (id: number, group: string) => (
    <span
      className="btn-icon !h-7 !w-7 cursor-grab select-none active:cursor-grabbing"
      draggable
      title="끌어서 순서 바꾸기 (↑/↓ 키로도 이동)"
      tabIndex={0}
      onDragStart={(e) => {
        setDrag({ id, group });
        e.dataTransfer.effectAllowed = "move";
        // Firefox 는 데이터가 없으면 드래그를 시작하지 않는다
        e.dataTransfer.setData("text/plain", String(id));
      }}
      onDragEnd={() => {
        setDrag(null);
        setOverId(null);
      }}
      onKeyDown={(e) => {
        if (e.key === "ArrowUp" || e.key === "ArrowDown") {
          e.preventDefault();
          run(() => MoveCategory(id, e.key === "ArrowUp" ? -1 : 1));
        }
      }}
    >
      <GripVertical className="h-4 w-4" />
    </span>
  );

  const allMains = mainCategories(refs.categories);

  const KIND_BADGE: Record<string, string> = {
    income: "bg-emerald-500/15 text-emerald-300",
    expense: "bg-rose-500/15 text-rose-300",
    transfer: "bg-sky-500/15 text-sky-300",
  };
  const OVER = "bg-mint-400/10 ring-1 ring-mint-400/40";

  return (
    <div className="surface space-y-5 p-5">
      {confirmUI}
      <SectionHead
        title="카테고리 (주/부)"
        desc="주 아래에 부를 두면 통계·예산이 주 기준으로 합산됩니다(부 단위 상세는 통계 탭에서 토글). 거래는 주·부 어디에나 분류할 수 있습니다."
      />
      {err && <p className="text-sm text-rose-300">{err}</p>}

      {Object.entries(KIND_LABEL).map(([kind, label]) => {
        const tree = treeOf(kind);
        if (tree.length === 0) return null;
        return (
          <div className="space-y-2" key={kind}>
            <h4 className="flex items-center gap-2 text-sm font-semibold text-white">
              <span className={`badge ${KIND_BADGE[kind]}`}>{label}</span>
              <span className="label-mono">{tree.length}개 주 카테고리</span>
            </h4>
            <ul className="space-y-2">
              {tree.map(({ main, subs }, mainIdx) => (
                <li className="surface-inset p-2" key={main.id}>
                  <div
                    className={
                      "flex items-center gap-2 rounded-lg px-2 py-1.5" + (overId === main.id ? ` ${OVER}` : "")
                    }
                    {...dragProps(main.id, `main:${kind}`, mainIdx)}
                  >
                    {grip(main.id, `main:${kind}`)}
                    <span className="min-w-0 flex-1 truncate text-sm font-medium text-zinc-100">{main.name}</span>
                    {subs.length > 0 && (
                      <span className="label-mono">부 {subs.length}</span>
                    )}
                    <button
                      className="btn-ghost !px-2.5 !py-1 !text-xs"
                      title="부 카테고리 추가"
                      onClick={() => {
                        setAddingUnder(addingUnder === main.id ? null : main.id);
                        setSubName("");
                      }}
                    ><Plus className="h-3.5 w-3.5" />부</button>
                    <button
                      className="btn-icon hover:!text-rose-300"
                      title="삭제"
                      aria-label="삭제"
                      onClick={() => remove(main.name, main.id, subs.length)}
                    ><Trash2 className="h-4 w-4" /></button>
                  </div>

                  {addingUnder === main.id && (
                    <form className="flex items-center gap-2 px-2 py-1.5 pl-11" onSubmit={(e) => addSub(e, main.id)}>
                      <input
                        type="text"
                        className="input-field"
                        autoFocus
                        placeholder={`${main.name} 아래 부 카테고리명`}
                        value={subName}
                        onChange={(e) => setSubName(e.target.value)}
                      />
                      <button type="submit" className="btn-primary shrink-0">추가</button>
                      <button
                        type="button"
                        className="btn-ghost shrink-0"
                        onClick={() => setAddingUnder(null)}
                      >취소</button>
                    </form>
                  )}

                  {subs.map((s, subIdx) => (
                    <div
                      className={
                        "flex items-center gap-2 rounded-lg px-2 py-1.5 pl-6" + (overId === s.id ? ` ${OVER}` : "")
                      }
                      key={s.id}
                      {...dragProps(s.id, `sub:${main.id}`, subIdx)}
                    >
                      {grip(s.id, `sub:${main.id}`)}
                      <span className="flex min-w-0 flex-1 items-center gap-1.5 truncate text-sm text-zinc-300">
                        <CornerDownRight className="h-3.5 w-3.5 shrink-0 text-zinc-600" />
                        {s.name}
                      </span>
                      <select
                        className="input-field !w-auto !py-1.5 !text-xs"
                        value={String(s.parentId)}
                        onChange={(e) => run(() => SetCategoryParent(s.id, Number(e.target.value)))}
                        title="상위(주) 카테고리 이동"
                      >
                        {allMains
                          .filter((m) => m.kind === s.kind)
                          .map((m) => (
                            <option key={m.id} value={m.id}>{m.name}</option>
                          ))}
                        <option value="0">주 카테고리로 올리기</option>
                      </select>
                      <button
                        className="btn-icon hover:!text-rose-300"
                        title="삭제"
                        aria-label="삭제"
                        onClick={() => remove(s.name, s.id, 0)}
                      ><Trash2 className="h-4 w-4" /></button>
                    </div>
                  ))}
                </li>
              ))}
            </ul>
          </div>
        );
      })}

      <form className="flex flex-wrap items-center gap-2" onSubmit={addMain}>
        <input
          type="text"
          className="input-field !w-auto min-w-[10rem] flex-1"
          placeholder="새 주 카테고리명"
          value={mainName}
          onChange={(e) => setMainName(e.target.value)}
        />
        <select className="input-field !w-auto" value={mainKind} onChange={(e) => setMainKind(e.target.value)}>
          {Object.entries(KIND_LABEL).map(([k, v]) => (
            <option key={k} value={k}>{v}</option>
          ))}
        </select>
        <button type="submit" className="btn-primary">주 카테고리 추가</button>
      </form>
    </div>
  );
}

// 로컬 백업: 운영은 Supabase, 정기적으로 로컬 SQLite 파일로 스냅샷 백업된다.
function BackupSection() {
  const [last, setLast] = useState<main.BackupInfo | null>(null);
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState("");

  const load = useCallback(async () => {
    try {
      setLast(await GetLastBackup());
    } catch {
      setLast(null);
    }
  }, []);
  useEffect(() => {
    load();
  }, [load]);

  const backupNow = async () => {
    setBusy(true);
    setMsg("");
    try {
      const info = await BackupNow();
      setLast(info);
      const total = info.counts
        ? Object.values(info.counts).reduce((a, b) => a + b, 0)
        : 0;
      setMsg(`백업 완료 · ${total}건 · ${info.sizeKB}KB`);
    } catch (e: any) {
      setMsg(String(e));
    } finally {
      setBusy(false);
    }
  };

  const folder = last?.path ? last.path.replace(/[/\\][^/\\]+$/, "") : "";

  return (
    <div className="surface space-y-4 p-5">
      <SectionHead
        title="로컬 백업"
        desc="운영 데이터베이스는 Supabase입니다. 앱 실행 중 자동으로(시작 직후·6시간마다·종료 시) 로컬 SQLite 파일로 스냅샷 백업하며, 최근 7개를 보관합니다."
      />
      <div className="surface-inset space-y-1 p-4 text-sm">
        {last?.path ? (
          <>
            <div className="text-zinc-300">
              최근 백업 <strong className="num text-white">{last.time}</strong>
              {last.sizeKB > 0 && <span className="label-mono num"> · {last.sizeKB}KB</span>}
            </div>
            <div className="label-mono truncate" title={folder}>{folder}</div>
          </>
        ) : (
          <span className="text-zinc-500">아직 백업이 없습니다.</span>
        )}
      </div>
      <div>
        <button className="btn-primary" onClick={backupNow} disabled={busy}>
          {busy ? "백업 중…" : "지금 백업"}
        </button>
      </div>
      {msg && <p className="label-mono whitespace-normal">{msg}</p>}
    </div>
  );
}

// 설정 하위 메뉴. 한 번에 한 섹션만 넓게 보여 가독성을 높인다.
type Section =
  | "categories"
  | "members"
  | "payments"
  | "budget"
  | "rules"
  | "holidays"
  | "appearance"
  | "backup";

const SECTIONS: { key: Section; label: string; desc: string }[] = [
  { key: "categories", label: "카테고리", desc: "주/부 분류 체계" },
  { key: "members", label: "귀속자", desc: "가족 구성원" },
  { key: "payments", label: "결제수단", desc: "카드·현금·계좌" },
  { key: "budget", label: "예산", desc: "카테고리별 한도" },
  { key: "rules", label: "자동 분류 규칙", desc: "학습된 핑거프린트" },
  { key: "holidays", label: "공휴일", desc: "정기결제 결제일 미루기" },
  { key: "appearance", label: "모양", desc: "테마·카드 스타일·글꼴·효과" },
  { key: "backup", label: "로컬 백업", desc: "SQLite 스냅샷" },
];

export default function SettingsPage({
  refs,
  reload,
}: {
  refs: Refs;
  reload: () => void;
}) {
  const [section, setSection] = useState<Section>("categories");
  const [appearanceOpen, setAppearanceOpen] = useState(false);
  const [confirm, confirmUI] = useConfirm();

  const del = async (
    fn: (id: number) => Promise<void>,
    id: number,
    title: string,
    message: string
  ) => {
    if (!(await confirm({ title, message, danger: true, confirmLabel: "삭제" }))) return;
    await fn(id);
    reload();
  };

  const counts: Record<Section, number | null> = {
    categories: refs.categories.length,
    members: refs.members.length,
    payments: refs.paymentMethods.length,
    budget: null,
    rules: null,
    holidays: null,
    appearance: null,
    backup: null,
  };

  return (
    <div className="grid items-start gap-4 lg:grid-cols-[220px_minmax(0,1fr)]">
      {confirmUI}
      <nav className="surface flex flex-row gap-1 overflow-x-auto p-2 lg:flex-col lg:overflow-visible">
        {SECTIONS.map((s) => (
          <button
            key={s.key}
            className={`pill-item !rounded-xl text-left ${section === s.key ? "pill-item-active" : ""}`}
            onClick={() => setSection(s.key)}
          >
            <span className="flex items-center justify-between gap-2 text-sm">
              {s.label}
              {counts[s.key] !== null && <span className="chip !px-1.5 !py-0">{counts[s.key]}</span>}
            </span>
            <span className="label-mono mt-0.5 block !text-[10px]">{s.desc}</span>
          </button>
        ))}
      </nav>

      <div className="min-w-0">
        {section === "categories" && <CategorySection refs={refs} reload={reload} />}

        {section === "members" && (
          <div className="surface space-y-4 p-5">
            <SectionHead title="귀속자 (가족 구성원)" desc="돈이 누구를 위해 쓰였는지 구분하는 기준입니다." />
            <ul className="space-y-2">
              {refs.members.map((m) => (
                <li key={m.id} className="surface-inset flex items-center justify-between gap-2 px-4 py-2.5 text-sm text-zinc-100">
                  {m.name}
                  <button className="btn-icon hover:!text-rose-300" title="삭제" aria-label="삭제" onClick={() =>
                    del(DeleteMember, m.id, `"${m.name}"을(를) 귀속자에서 삭제할까요?`,
                      "이 사람으로 기록된 거래는 그대로 남고 귀속자만 비워집니다.")
                  }><Trash2 className="h-4 w-4" /></button>
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
        )}

        {section === "payments" && (
          <div className="surface space-y-4 p-5">
            <SectionHead
              title="결제수단 (카드/현금/계좌)"
              desc="카드의 결제일·실적기간·실적한도는 카드 탭에서 관리합니다."
            />
            <ul className="space-y-2">
              {refs.paymentMethods.map((p) => (
                <li key={p.id} className="surface-inset flex items-center justify-between gap-2 px-4 py-2.5 text-sm text-zinc-100">
                  <span className="flex items-center gap-2">
                    {p.name} <span className="chip">{PM_LABEL[p.type]}</span>
                  </span>
                  <button className="btn-icon hover:!text-rose-300" title="삭제" aria-label="삭제" onClick={() =>
                    del(DeletePaymentMethod, p.id, `"${p.name}" 결제수단을 삭제할까요?`,
                      "이 결제수단으로 기록된 거래는 그대로 남고 결제수단만 비워집니다.")
                  }><Trash2 className="h-4 w-4" /></button>
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
        )}

        {section === "budget" && <BudgetSection refs={refs} />}
        {section === "rules" && <RuleSection refs={refs} />}
        {section === "holidays" && <HolidaySection />}

        {section === "appearance" && (
          <div className="surface space-y-4 p-5">
            <SectionHead
              title="모양"
              desc="테마, 카드 스타일, 글꼴, 효과를 취향에 맞게 바꿀 수 있습니다. 설정은 이 기기에 저장됩니다."
            />
            <button className="btn-primary" onClick={() => setAppearanceOpen(true)}>
              <Palette className="h-4 w-4" />
              모양 설정 열기
            </button>
          </div>
        )}
        <AppearanceSettings open={appearanceOpen} onClose={() => setAppearanceOpen(false)} />

        {section === "backup" && <BackupSection />}
      </div>
    </div>
  );
}
