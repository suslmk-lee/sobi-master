import { useCallback, useEffect, useState } from "react";
import {
  AlertTriangle,
  BarChart3,
  Coins,
  CreditCard,
  LayoutDashboard,
  LucideIcon,
  Palette,
  ReceiptText,
  Repeat,
  Settings,
  Wallet,
} from "lucide-react";
import {
  ListCategories,
  ListMembers,
  ListPaymentMethods,
} from "../wailsjs/go/main/App";
import { Refs, thisMonth } from "./lib";
import Dashboard from "./pages/Dashboard";
import Transactions from "./pages/Transactions";
import CardsPage from "./pages/CardsPage";
import CashPlanPage from "./pages/CashPlanPage";
import SubscriptionsPage from "./pages/SubscriptionsPage";
import CashbackPage from "./pages/CashbackPage";
import StatsPage from "./pages/StatsPage";
import SettingsPage from "./pages/SettingsPage";
import Brand from "./components/ui/Brand";
import IconTile from "./components/ui/IconTile";
import AppearanceSettings from "./components/settings/AppearanceSettings";

type Tab =
  | "dashboard"
  | "cashplan"
  | "transactions"
  | "cards"
  | "subscriptions"
  | "cashback"
  | "stats"
  | "settings";

const TABS: { key: Tab; label: string; icon: LucideIcon }[] = [
  { key: "dashboard", label: "대시보드", icon: LayoutDashboard },
  { key: "cashplan", label: "자금 계획", icon: Wallet },
  { key: "transactions", label: "거래내역", icon: ReceiptText },
  { key: "cards", label: "카드", icon: CreditCard },
  { key: "subscriptions", label: "정기결제", icon: Repeat },
  { key: "cashback", label: "캐시백", icon: Coins },
  { key: "stats", label: "통계", icon: BarChart3 },
];

export default function App() {
  const [tab, setTab] = useState<Tab>("dashboard");
  const [month, setMonth] = useState(thisMonth());
  const [unclassifiedOnly, setUnclassifiedOnly] = useState(false);
  const [appearanceOpen, setAppearanceOpen] = useState(false);
  const [refs, setRefs] = useState<Refs>({
    members: [],
    categories: [],
    paymentMethods: [],
  });
  const [dbError, setDbError] = useState("");

  const loadRefs = useCallback(async () => {
    try {
      const [members, categories, paymentMethods] = await Promise.all([
        ListMembers(),
        ListCategories(),
        ListPaymentMethods(),
      ]);
      // 카테고리 순서는 백엔드가 정한 그대로 쓴다(설정에서 지정한 표시 순서).
      // 종류 → 주의 순서 → 그 주의 부들 순서로 이미 정렬돼 오므로 여기서 다시 섞지 않는다.
      setRefs({ members, categories, paymentMethods });
      setDbError("");
    } catch (e: any) {
      setDbError(String(e));
    }
  }, []);

  useEffect(() => {
    loadRefs();
  }, [loadRefs]);

  return (
    <div className="relative min-h-screen app-bg text-zinc-100">
      {/* 상단 은은한 광원 */}
      <div
        aria-hidden
        className="pointer-events-none fixed inset-x-0 top-0 h-[420px]"
        style={{
          background:
            "radial-gradient(60% 100% at 50% 0%, rgb(var(--c-white) / 0.06) 0%, transparent 70%)",
        }}
      />

      <header className="app-header sticky top-0 z-20 backdrop-blur-xl">
        <div className="mx-auto max-w-7xl px-4 pb-3 pt-4 sm:px-6 lg:px-8">
          <div className="mb-4 flex items-center justify-between gap-3">
            <Brand />
            <div className="flex items-center gap-2">
              {/* 설정은 탭이 아니라 헤더의 작은 기어 아이콘으로 들어간다 (열려 있으면 민트로 표시) */}
              <button
                type="button"
                onClick={() => setTab("settings")}
                title="설정"
                aria-label="설정"
                aria-current={tab === "settings" ? "page" : undefined}
                className={`glass-pill flex h-9 w-9 items-center justify-center transition-colors ${
                  tab === "settings" ? "text-mint-300" : "text-zinc-400 hover:text-white"
                }`}
              >
                <Settings className="h-4 w-4" />
              </button>
              <button
                type="button"
                onClick={() => setAppearanceOpen(true)}
                title="모양 설정"
                aria-label="모양 설정"
                className="glass-pill flex h-9 w-9 items-center justify-center text-zinc-400 transition-colors hover:text-white"
              >
                <Palette className="h-4 w-4" />
              </button>
            </div>
          </div>

          <nav className="-mb-1 flex gap-2 overflow-x-auto pb-1 scrollbar-hide">
            {TABS.map((t) => {
              const active = tab === t.key;
              return (
                <button
                  key={t.key}
                  type="button"
                  onClick={() => setTab(t.key)}
                  aria-current={active ? "page" : undefined}
                  className={`tab-card ${active ? "tab-card-active" : ""}`}
                >
                  <div className="flex w-full items-start justify-between">
                    <IconTile icon={t.icon} size="sm" tone={active ? "mint" : "warm"} />
                  </div>
                  <span
                    className={`text-sm font-medium leading-tight ${active ? "text-white" : "text-zinc-400"}`}
                  >
                    {t.label}
                  </span>
                  <span className="absolute inset-x-3.5 bottom-2 h-[2px] overflow-hidden rounded-full bg-white/[0.06]">
                    {active && (
                      <span
                        className="block h-full w-1/2 origin-left rounded-full bg-white/80"
                        style={{ animation: "tabProgress 0.6s cubic-bezier(0.22, 1, 0.36, 1) both" }}
                      />
                    )}
                  </span>
                </button>
              );
            })}
          </nav>
        </div>
        <div className="h-px bg-gradient-to-r from-transparent via-white/[0.08] to-transparent" />
      </header>

      {/* 탭 전환 시 각 화면이 마운트되며 데이터를 새로 불러온다 */}
      <main
        key={tab}
        className="relative mx-auto max-w-7xl animate-fade-up px-4 py-6 sm:px-6 sm:py-8 lg:px-8"
      >
        {dbError && (
          <div className="surface mb-5 p-4 sm:p-5" role="alert">
            <div className="flex items-start gap-3">
              <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-rose-400/30 bg-rose-500/10 text-rose-300">
                <AlertTriangle className="h-4 w-4" strokeWidth={1.75} />
              </span>
              <div className="min-w-0 flex-1">
                <strong className="text-sm font-medium text-rose-200">데이터베이스 연결 오류</strong>
                <p className="mt-1 break-words text-xs text-zinc-300">{dbError}</p>
                <p className="mt-1 text-xs text-zinc-500">
                  config.json 의 database_url 과 인터넷 연결을 확인한 뒤 다시 시도하세요.
                  상세 내용은 설정 폴더의 sobi.log 에 기록됩니다.
                </p>
                <button type="button" className="btn-primary mt-3" onClick={loadRefs}>
                  다시 연결
                </button>
              </div>
            </div>
          </div>
        )}
        {tab === "dashboard" && (
          <Dashboard
            month={month}
            setMonth={setMonth}
            goCashPlan={() => setTab("cashplan")}
            goUnclassified={() => {
              setUnclassifiedOnly(true);
              setTab("transactions");
            }}
          />
        )}
        {tab === "cashplan" && <CashPlanPage refs={refs} />}
        {tab === "transactions" && (
          <Transactions
            refs={refs}
            month={month}
            setMonth={setMonth}
            unclassifiedOnly={unclassifiedOnly}
            setUnclassifiedOnly={setUnclassifiedOnly}
          />
        )}
        {tab === "cards" && <CardsPage reloadRefs={loadRefs} />}
        {tab === "subscriptions" && (
          <SubscriptionsPage refs={refs} month={month} setMonth={setMonth} />
        )}
        {tab === "cashback" && <CashbackPage month={month} setMonth={setMonth} />}
        {tab === "stats" && <StatsPage refs={refs} month={month} setMonth={setMonth} />}
        {tab === "settings" && <SettingsPage refs={refs} reload={loadRefs} />}
      </main>

      <AppearanceSettings open={appearanceOpen} onClose={() => setAppearanceOpen(false)} />
    </div>
  );
}
