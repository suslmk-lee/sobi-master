import { useCallback, useEffect, useRef, useState } from "react";
import { AlertTriangle, Bell, ChevronRight, Info, Siren } from "lucide-react";
import { GetAlerts } from "../../wailsjs/go/main/App";
import { main } from "../../wailsjs/go/models";

// 헤더의 종 버튼과 알림창.
// 예산 초과·임박, 카드 실적 미달, 회비 미납·확인 필요, 미분류 거래를 모아 보여 준다(백엔드 alerts.go).
// 배지는 아직 안 본 알림 수 — 알림창을 열면 지금 있는 알림을 본 것으로 기억한다(이 기기에만).
// 상황이 나빠지면(예산 임박 → 초과, 밀린 달이 늘어남) 알림 키가 바뀌어 다시 새 알림으로 센다.
// 알림을 누르면 관련 화면으로 간다.

const SEEN_KEY = "sobi.alerts.seen";
const REFRESH_MS = 10 * 60 * 1000;

function loadSeen(): Set<string> {
  try {
    const v = JSON.parse(localStorage.getItem(SEEN_KEY) ?? "[]");
    return new Set(Array.isArray(v) ? v : []);
  } catch {
    return new Set();
  }
}

function saveSeen(keys: string[]) {
  try {
    localStorage.setItem(SEEN_KEY, JSON.stringify(keys));
  } catch {
    // 기억하지 못해도 알림창은 그대로 쓴다
  }
}

const LEVEL: Record<string, { Icon: typeof Info; cls: string }> = {
  danger: { Icon: Siren, cls: "text-rose-300" },
  warn: { Icon: AlertTriangle, cls: "text-amber-300" },
  info: { Icon: Info, cls: "text-sky-300" },
};

export default function AlertBell({ refreshKey, onGo }: { refreshKey: string; onGo: (kind: string) => void }) {
  const [alerts, setAlerts] = useState<main.Alert[]>([]);
  const [seen, setSeen] = useState<Set<string>>(loadSeen);
  const [fresh, setFresh] = useState<Set<string>>(new Set()); // 이번에 열었을 때 새것이던 알림 (점 표시)
  const [open, setOpen] = useState(false);
  const boxRef = useRef<HTMLDivElement>(null);

  const load = useCallback(() => {
    GetAlerts()
      .then((a) => setAlerts(a ?? []))
      .catch(() => {});
  }, []);

  // 화면을 옮길 때마다(데이터가 바뀌었을 수 있다), 창으로 돌아올 때, 10분마다 다시 읽는다
  useEffect(() => {
    load();
  }, [load, refreshKey]);
  useEffect(() => {
    const t = window.setInterval(load, REFRESH_MS);
    window.addEventListener("focus", load);
    return () => {
      window.clearInterval(t);
      window.removeEventListener("focus", load);
    };
  }, [load]);

  // 열려 있는 동안의 알림은 본 것으로 기억한다 (지금 있는 것만 남겨 목록이 끝없이 늘지 않게)
  useEffect(() => {
    if (!open) return;
    const keys = alerts.map((a) => a.key);
    saveSeen(keys);
    setSeen(new Set(keys));
  }, [open, alerts]);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (boxRef.current && !boxRef.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onDown);
    window.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDown);
      window.removeEventListener("keydown", onKey);
    };
  }, [open]);

  const unseen = alerts.filter((a) => !seen.has(a.key));
  const toggle = () => {
    if (!open) setFresh(new Set(unseen.map((a) => a.key)));
    setOpen(!open);
  };

  return (
    <div ref={boxRef} className="relative">
      <button
        type="button"
        onClick={toggle}
        title={alerts.length ? `알림 ${alerts.length}건` : "알림"}
        aria-label={unseen.length ? `알림, 새 알림 ${unseen.length}건` : "알림"}
        aria-expanded={open}
        className={`glass-pill relative flex h-9 w-9 items-center justify-center transition-colors hover:text-white ${
          open ? "text-mint-300" : alerts.length ? "text-zinc-200" : "text-zinc-400"
        }`}
      >
        <Bell className="h-4 w-4" />
        {unseen.length > 0 && (
          // 배지는 테마와 상관없이 선명한 빨강 (라이트 테마의 상태색은 글자용으로 어둡게 섞여 탁하다)
          <span className="num absolute -right-1.5 -top-1.5 flex h-[18px] min-w-[18px] items-center justify-center rounded-full bg-[#ef4444] px-1 text-[10px] font-semibold leading-none text-white">
            {unseen.length > 9 ? "9+" : unseen.length}
          </span>
        )}
      </button>

      {open && (
        <div
          role="dialog"
          aria-label="알림"
          className="surface absolute right-0 top-11 z-30 w-[min(400px,calc(100vw-2rem))] p-2"
        >
          <div className="flex items-center justify-between px-3 pb-2 pt-1.5">
            <span className="text-sm font-medium text-white">알림</span>
            {alerts.length > 0 && <span className="num text-xs text-zinc-500">{alerts.length}건</span>}
          </div>
          {alerts.length === 0 ? (
            <p className="px-3 pb-5 pt-3 text-center text-sm text-zinc-500">지금 알릴 것이 없습니다</p>
          ) : (
            <ul className="max-h-[60vh] space-y-0.5 overflow-y-auto">
              {alerts.map((a) => {
                const L = LEVEL[a.level] ?? LEVEL.info;
                return (
                  <li key={a.key}>
                    <button
                      type="button"
                      onClick={() => {
                        setOpen(false);
                        onGo(a.kind);
                      }}
                      className="flex w-full items-start gap-3 rounded-xl px-3 py-2.5 text-left transition-colors hover:bg-white/[0.05]"
                    >
                      <L.Icon className={`mt-0.5 h-4 w-4 shrink-0 ${L.cls}`} />
                      <span className="min-w-0 flex-1">
                        <span className="flex items-center gap-1.5 text-sm text-white">
                          {a.title}
                          {fresh.has(a.key) && (
                            <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-rose-400" aria-label="새 알림" />
                          )}
                        </span>
                        <span className="num mt-0.5 block text-xs text-zinc-400">{a.detail}</span>
                      </span>
                      <ChevronRight className="mt-0.5 h-4 w-4 shrink-0 text-zinc-500" />
                    </button>
                  </li>
                );
              })}
            </ul>
          )}
        </div>
      )}
    </div>
  );
}
