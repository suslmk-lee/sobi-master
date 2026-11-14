// frontend/src/components/icons.tsx
// 탭 · 섹션 머리 · 설정 묶음 · 팝업 제목 아이콘 :: 퀀텀메이트(client/src/icons.tsx)와 같은 "어긋난 두 색".
// 직접 그린 24 격자 선(글자색) 위로, 테마색 면을 오른쪽 아래로 1.2 어긋나게 겹친다.
// 예전에는 lucide 아이콘을 어두운 그라데이션 타일에 담았는데(템플릿처럼 보임) 이걸로 바꿨다.
// 버튼 안 · 목록 줄의 작은 기능 아이콘(닫기 · 추가 · 화살표 · 이벤트 종류)은 lucide 그대로 쓴다.
// 퀀텀메이트에 같은 개념이 있으면 그 그림을 그대로 쓴다(대시보드 카드 · 달력 · 추세 · 사람 · 설정 · 시계 …).
// l = 선, f = 면. 문자열은 이 파일의 상수만 넣는다(dangerouslySetInnerHTML).
import {
  Activity, ArrowDownLeft, ArrowRightLeft, BarChart3, Box, Calendar, CalendarClock, CalendarDays, CalendarOff,
  CalendarRange, Clock, Coins, Contrast, CreditCard, Flag, Gauge, GitCompare, Globe, Grid3x3, History, Layers,
  LayoutDashboard, LayoutTemplate, MinusCircle, MousePointer2, Palette, Pencil, PieChart, PiggyBank, PlusCircle,
  Receipt, ReceiptText, RefreshCw, Repeat, ScanSearch, Settings, Settings2, Sparkles, SunMedium, Target, Ticket,
  TrainFront, TrendingDown, TrendingUp, Type, UserRound, Users, Wallet, type LucideIcon,
} from 'lucide-react';
import type { ReactElement } from 'react';

export type DuoIcon = (p: { className?: string; fill?: string }) => ReactElement;

export const ACCENT_FILL = 'rgb(var(--c-mint-400))';

const make =
  (l: string, f: string): DuoIcon =>
  ({ className, fill = ACCENT_FILL }) => (
    <svg viewBox="0 0 24 24" className={className} aria-hidden="true">
      <g fill={fill} opacity={0.7} transform="translate(1.2 1.2)" dangerouslySetInnerHTML={{ __html: f }} />
      <g
        fill="none"
        stroke="currentColor"
        strokeWidth={1.6}
        strokeLinecap="round"
        strokeLinejoin="round"
        dangerouslySetInnerHTML={{ __html: l }}
      />
    </svg>
  );

// ---- 탭
export const IconDashboard = make(
  '<rect x="3.5" y="3.5" width="7.5" height="9" rx="1.5"/><rect x="13" y="3.5" width="7.5" height="5.5" rx="1.5"/><rect x="13" y="11" width="7.5" height="9.5" rx="1.5"/><rect x="3.5" y="14.5" width="7.5" height="6" rx="1.5"/>',
  '<rect x="3.5" y="3.5" width="7.5" height="9" rx="1.5"/>',
);
export const IconWallet = make(
  '<path d="M17.5 6.5V5.5A1.5 1.5 0 0 0 16 4H6a2.5 2.5 0 0 0-2.5 2.5"/><rect x="3.5" y="6.5" width="17" height="13.5" rx="2"/><path d="M20.5 11h-4a2.25 2.25 0 0 0 0 4.5h4"/>',
  '<rect x="14.5" y="11" width="6" height="4.5" rx="1.5"/>',
);
export const IconReceipt = make(
  '<path d="M5.5 3.5h13v17l-2.2-1.4-2.1 1.4-2.2-1.4-2.2 1.4-2.1-1.4-2.2 1.4z"/><path d="M8.5 8h7M8.5 11.5h7M8.5 15h3"/>',
  '<rect x="12.5" y="13.5" width="4" height="3" rx="1"/>',
);
export const IconCard = make(
  '<rect x="3" y="5.5" width="18" height="13" rx="2"/><path d="M3 9.5h18M6.5 14.5h4"/>',
  '<rect x="13.5" y="12.5" width="4.5" height="3.5" rx="1"/>',
);
export const IconRepeat = make(
  '<path d="M4.5 11V9.5A3.5 3.5 0 0 1 8 6h11.5M17 3.5 19.5 6 17 8.5M19.5 13v1.5A3.5 3.5 0 0 1 16 18H4.5M7 20.5 4.5 18 7 15.5"/>',
  '<circle cx="12" cy="12" r="2.5"/>',
);
export const IconCoins = make(
  '<circle cx="12" cy="12" r="8.5"/><path d="M7.8 8.5l1.8 7 2.4-5 2.4 5 1.8-7M7.2 12h9.6"/>',
  '<path d="M12 3.5a8.5 8.5 0 0 1 0 17z"/>',
);
export const IconGlobe = make(
  '<circle cx="12" cy="12" r="8.5"/><path d="M3.5 12h17M12 3.5c2.2 2.4 3.3 5.2 3.3 8.5s-1.1 6.1-3.3 8.5c-2.2-2.4-3.3-5.2-3.3-8.5S9.8 5.9 12 3.5z"/>',
  '<path d="M12 3.5c2.2 2.4 3.3 5.2 3.3 8.5s-1.1 6.1-3.3 8.5c-2.2-2.4-3.3-5.2-3.3-8.5S9.8 5.9 12 3.5z"/>',
);
export const IconBars = make(
  '<path d="M3.5 20h17M7 16.5V12M12 16.5V6.5M17 16.5V9.5"/>',
  '<rect x="10.5" y="6.5" width="3" height="10" rx="1"/>',
);

export const IconTrain = make(
  '<rect x="5.5" y="3.5" width="13" height="14" rx="3"/><path d="M5.5 11h13M9 20.5l1-3M15 20.5l-1-3"/><circle cx="9" cy="14.3" r=".9"/><circle cx="15" cy="14.3" r=".9"/>',
  '<rect x="5.5" y="3.5" width="13" height="7.5" rx="3"/>',
);

// ---- 달력 · 시간
export const IconCalendar = make(
  '<rect x="3.5" y="5" width="17" height="15" rx="2"/><path d="M3.5 9.5h17M8 3v4M16 3v4M7.5 13h2M11 13h2M14.5 13h2M7.5 16.5h2M11 16.5h2"/>',
  '<rect x="14" y="15" width="3.5" height="3" rx="1"/>',
);
export const IconCalendarClock = make(
  '<path d="M20.5 11V7a2 2 0 0 0-2-2h-13a2 2 0 0 0-2 2v11a2 2 0 0 0 2 2H11M3.5 9.5h17M8 3v4M16 3v4"/><circle cx="17" cy="17" r="4"/><path d="M17 15.3V17l1.2 1"/>',
  '<circle cx="17" cy="17" r="3.5"/>',
);
export const IconCalendarOff = make(
  '<rect x="3.5" y="5" width="17" height="15" rx="2"/><path d="M3.5 9.5h17M8 3v4M16 3v4M9.5 12.5l5 5M14.5 12.5l-5 5"/>',
  '<path d="M5.5 5h13a2 2 0 0 1 2 2v2.5h-17V7a2 2 0 0 1 2-2z"/>',
);
export const IconCalendarRange = make(
  '<rect x="3.5" y="5" width="17" height="15" rx="2"/><path d="M3.5 9.5h17M8 3v4M16 3v4M7.5 14.5h9"/>',
  '<rect x="7" y="13" width="10" height="3.5" rx="1.75"/>',
);
export const IconClock = make('<circle cx="12" cy="12.5" r="8"/><path d="M12 8.5v4l2.5 2"/>', '<path d="M12 4.5a8 8 0 0 1 8 8h-8z"/>');
export const IconHistory = make(
  '<path d="M3.8 12a8.5 8.5 0 1 0 2.5-6L3.8 8.5M3.8 3.8v4.7h4.7"/><path d="M12 8v4.2l3 1.8"/>',
  '<path d="M12 3.5a8.5 8.5 0 0 1 8.5 8.5H12z"/>',
);

// ---- 추세 · 그래프
export const IconTrendUp = make('<path d="M3.5 17.5l5-5 3.5 3.5 8-8M15 8h5v5"/>', '<path d="M4 20l4.5-5 3.5 3.5 7.5-7.5V20z"/>');
export const IconTrendDown = make('<path d="M3.5 6.5l5 5 3.5-3.5 8 8M15 16h5v-5"/>', '<path d="M4 9.5l4.5 4.5 3.5-3.5 7.5 7.5V20H4z"/>');
export const IconActivity = make('<path d="M3 12.5h4l2.5-6 4 12 2.5-6h5"/>', '<circle cx="16" cy="12.5" r="2.5"/>');
export const IconPie = make(
  '<path d="M20.5 13.5A8.5 8.5 0 1 1 10.5 3.6"/><path d="M13.5 3.5a7 7 0 0 1 7 7h-7z"/>',
  '<path d="M13.5 3.5a7 7 0 0 1 7 7h-7z"/>',
);
export const IconCompare = make(
  '<path d="M3.5 20.5h17"/><rect x="5" y="10" width="5.5" height="10.5" rx="1.5"/><rect x="13.5" y="4.5" width="5.5" height="16" rx="1.5"/>',
  '<rect x="5" y="10" width="5.5" height="10.5" rx="1.5"/>',
);
export const IconGrid = make(
  '<rect x="3.5" y="4.5" width="17" height="15" rx="2"/><path d="M3.5 9.5h17M3.5 14.5h17M9 4.5v15M15 4.5v15"/>',
  '<rect x="9.5" y="10" width="5" height="4" rx="1"/>',
);
export const IconLayers = make(
  '<path d="M12 4l8.5 4.5L12 13 3.5 8.5z"/><path d="M3.5 12.5 12 17l8.5-4.5M3.5 16.5 12 21l8.5-4.5"/>',
  '<path d="M12 4l8.5 4.5L12 13 3.5 8.5z"/>',
);
export const IconGauge = make(
  '<path d="M4.2 16.5a8.5 8.5 0 1 1 15.6 0"/><path d="M12 14l3.5-4.5"/><circle cx="12" cy="14.5" r="1.2"/>',
  '<path d="M3.5 13a8.5 8.5 0 0 1 4.3-7.4L12 14z"/>',
);
export const IconTarget = make('<circle cx="12" cy="12" r="8.5"/><circle cx="12" cy="12" r="4.5"/>', '<circle cx="12" cy="12" r="2.2"/>');
export const IconFlag = make('<path d="M5.5 20.5v-16M5.5 4.5h11l-2 4 2 4h-11"/>', '<path d="M5.5 4.5h11l-2 4 2 4h-11z"/>');

// ---- 돈의 흐름
export const IconArrowIn = make('<path d="M17.5 6.5l-11 11M6.5 9.5v8h8"/>', '<path d="M6.5 17.5V12l5.5 5.5z"/>');
export const IconTransfer = make('<path d="M16 3.5l4 4-4 4M20 7.5H8M8 12.5l-4 4 4 4M4 16.5h12"/>', '<path d="M16 3.5l4 4-4 4z"/>');
export const IconPiggy = make(
  '<path d="M19 9.5c.9.3 1.5 1 1.5 2V13h-1.6a6 6 0 0 1-2.4 3v3h-2.5v-1.6a8 8 0 0 1-3 0V19H8.5v-2.6A5.8 5.8 0 0 1 5 11c0-3.3 3.1-5.5 7-5.5 1.7 0 3.2.4 4.4 1.1l2.1-1.1z"/><path d="M10 8.5h3"/>',
  '<circle cx="9.5" cy="11.5" r="3"/>',
);
export const IconTicket = make(
  '<path d="M3.5 7.5A1.5 1.5 0 0 1 5 6h14a1.5 1.5 0 0 1 1.5 1.5V10a2 2 0 0 0 0 4v2.5A1.5 1.5 0 0 1 19 18H5a1.5 1.5 0 0 1-1.5-1.5V14a2 2 0 0 0 0-4z"/><path d="M14.5 6v2.5M14.5 10.75v2.5M14.5 15.5V18"/>',
  '<rect x="6.5" y="9" width="5" height="6" rx="1"/>',
);
export const IconPlus = make('<circle cx="12" cy="12" r="8.5"/><path d="M12 8.5v7M8.5 12h7"/>', '<circle cx="12" cy="12" r="5"/>');
export const IconMinus = make('<circle cx="12" cy="12" r="8.5"/><path d="M8.5 12h7"/>', '<circle cx="12" cy="12" r="5"/>');

// ---- 사람
export const IconTeam = make(
  '<circle cx="9" cy="8.5" r="3.5"/><path d="M3 19.5c.5-3.3 3-5.5 6-5.5s5.5 2.2 6 5.5M15.5 5.3a3.5 3.5 0 0 1 0 6.4M17.5 14.3c1.8.8 3 2.6 3.5 5.2"/>',
  '<circle cx="9" cy="8.5" r="3.5"/>',
);
export const IconMe = make(
  '<circle cx="12" cy="12" r="8.5"/><circle cx="12" cy="10" r="3"/><path d="M6.5 18.5c1.2-2.2 3.2-3.5 5.5-3.5s4.3 1.3 5.5 3.5"/>',
  '<circle cx="12" cy="10" r="3"/>',
);

// ---- 설정 · 꾸미기 · 도구
export const IconGear = make(
  '<circle cx="12" cy="12" r="3"/><path d="M12 3.5v3M12 17.5v3M3.5 12h3M17.5 12h3M6 6l2.1 2.1M15.9 15.9 18 18M6 18l2.1-2.1M15.9 8.1 18 6"/>',
  '<circle cx="12" cy="12" r="5"/>',
);
export const IconSliders = make(
  '<path d="M4 7h9M17 7h3M4 12h3M11 12h9M4 17h11M19 17h1"/><circle cx="15" cy="7" r="2"/><circle cx="9" cy="12" r="2"/><circle cx="17" cy="17" r="2"/>',
  '<circle cx="9" cy="12" r="2.5"/>',
);
export const IconPalette = make(
  '<path d="M12 3.5a8.5 8.5 0 0 0 0 17c1.3 0 2-.9 1.6-2.1-.5-1.4.3-2.4 1.7-2.4H18a2.5 2.5 0 0 0 2.5-2.5c0-5.5-3.8-10-8.5-10z"/><circle cx="7.8" cy="11.5" r="1"/><circle cx="10.5" cy="7.5" r="1"/><circle cx="15.2" cy="8" r="1"/>',
  '<circle cx="15.2" cy="8" r="2.2"/>',
);
export const IconTemplate = make(
  '<rect x="3.5" y="4" width="17" height="7" rx="1.5"/><rect x="3.5" y="13.5" width="7.5" height="6.5" rx="1.5"/><rect x="13.5" y="13.5" width="7" height="6.5" rx="1.5"/>',
  '<rect x="3.5" y="4" width="17" height="7" rx="1.5"/>',
);
export const IconType = make('<path d="M5 7V4.5h14V7M12 4.5v15M9 19.5h6"/>', '<rect x="5" y="4.5" width="14" height="3" rx="1"/>');
export const IconContrast = make('<circle cx="12" cy="12" r="8.5"/><path d="M12 3.5v17"/>', '<path d="M12 3.5a8.5 8.5 0 0 1 0 17z"/>');
export const IconSun = make(
  '<circle cx="12" cy="12" r="4"/><path d="M12 3.5v2M12 18.5v2M3.5 12h2M18.5 12h2M6 6l1.4 1.4M16.6 16.6 18 18M6 18l1.4-1.4M16.6 7.4 18 6"/>',
  '<circle cx="12" cy="12" r="4"/>',
);
export const IconPointer = make('<path d="M4.5 4.5l15 6-6.3 1.7-1.7 6.3z"/>', '<path d="M4.5 4.5l15 6-6.3 1.7-1.7 6.3z"/>');
export const IconBox = make(
  '<path d="M12 3.5l7.5 4.3v8.4L12 20.5l-7.5-4.3V7.8z"/><path d="M12 12l7.5-4.2M12 12v8.5M12 12 4.5 7.8"/>',
  '<path d="M12 3.5l7.5 4.3L12 12 4.5 7.8z"/>',
);
export const IconPencil = make('<path d="M15.5 4.5l4 4L8.5 19.5h-4v-4z"/><path d="M13.5 6.5l4 4"/>', '<path d="M15.5 4.5l4 4-2 2-4-4z"/>');
export const IconScan = make(
  '<path d="M3.5 8V5.5a2 2 0 0 1 2-2H8M16 3.5h2.5a2 2 0 0 1 2 2V8M20.5 16v2.5a2 2 0 0 1-2 2H16M8 20.5H5.5a2 2 0 0 1-2-2V16"/><circle cx="11.5" cy="11.5" r="3.5"/><path d="M14 14l2.5 2.5"/>',
  '<circle cx="11.5" cy="11.5" r="3.5"/>',
);
export const IconRefresh = make(
  '<path d="M20 11.5A8 8 0 0 0 6.3 6.3L3.5 9M3.5 4.5V9H8M4 12.5a8 8 0 0 0 13.7 5.2l2.8-2.7M20.5 19.5V15H16"/>',
  '<circle cx="12" cy="12" r="3"/>',
);
export const IconSpark = make(
  '<path d="M12 3.5l2 6 6 2-6 2-2 6-2-6-6-2 6-2z"/><path d="M18.5 3.5v3M17 5h3"/>',
  '<path d="M12 7.5l1 3.5 3.5 1-3.5 1-1 3.5-1-3.5-3.5-1 3.5-1z"/>',
);

// 호출부는 예전처럼 lucide 아이콘을 넘기고(IconTile), 여기서 같은 뜻의 두 색 아이콘으로 바꾼다.
const DUO = new Map<LucideIcon, DuoIcon>([
  [LayoutDashboard, IconDashboard], [Wallet, IconWallet], [Receipt, IconReceipt], [ReceiptText, IconReceipt],
  [CreditCard, IconCard], [Repeat, IconRepeat], [Coins, IconCoins], [Globe, IconGlobe], [BarChart3, IconBars],
  [Calendar, IconCalendar], [CalendarDays, IconCalendar], [CalendarClock, IconCalendarClock],
  [CalendarOff, IconCalendarOff], [CalendarRange, IconCalendarRange], [Clock, IconClock], [History, IconHistory],
  [TrendingUp, IconTrendUp], [TrendingDown, IconTrendDown], [Activity, IconActivity], [PieChart, IconPie],
  [GitCompare, IconCompare], [Grid3x3, IconGrid], [Layers, IconLayers], [Gauge, IconGauge], [Target, IconTarget],
  [Flag, IconFlag], [ArrowDownLeft, IconArrowIn], [ArrowRightLeft, IconTransfer], [PiggyBank, IconPiggy],
  [Ticket, IconTicket], [PlusCircle, IconPlus], [MinusCircle, IconMinus], [Users, IconTeam], [UserRound, IconMe],
  [Settings, IconGear], [Settings2, IconSliders], [Palette, IconPalette], [LayoutTemplate, IconTemplate],
  [Type, IconType], [Contrast, IconContrast], [SunMedium, IconSun], [MousePointer2, IconPointer], [Box, IconBox],
  [Pencil, IconPencil], [ScanSearch, IconScan], [RefreshCw, IconRefresh], [Sparkles, IconSpark], [TrainFront, IconTrain],
]);

// 짝이 없는 아이콘은 lucide 선만 같은 굵기로 그린다(면 없이) — 새 화면에서 짝을 빠뜨려도 깨지지 않게
export function duoFor(icon: LucideIcon): DuoIcon {
  const found = DUO.get(icon);
  if (found) return found;
  const Fallback = icon;
  return ({ className }) => <Fallback className={className} strokeWidth={1.6} />;
}

// 시안 · 점검용 목록 (짝이 있는 lucide 아이콘 수)
export const DUO_COUNT = DUO.size;
export const ALL_DUO: [string, DuoIcon][] = [
  ['대시보드', IconDashboard], ['지갑', IconWallet], ['영수증', IconReceipt], ['카드', IconCard], ['정기결제', IconRepeat],
  ['동전', IconCoins], ['외화', IconGlobe], ['통계', IconBars], ['달력', IconCalendar], ['달력·시계', IconCalendarClock],
  ['쉬는 날', IconCalendarOff], ['기간', IconCalendarRange], ['시계', IconClock], ['기록', IconHistory],
  ['오름', IconTrendUp], ['내림', IconTrendDown], ['활동', IconActivity], ['비중', IconPie], ['비교', IconCompare],
  ['히트맵', IconGrid], ['층', IconLayers], ['게이지', IconGauge], ['목표', IconTarget], ['깃발', IconFlag],
  ['들어온 돈', IconArrowIn], ['주고받기', IconTransfer], ['저금통', IconPiggy], ['티켓', IconTicket],
  ['더하기', IconPlus], ['빼기', IconMinus], ['사람들', IconTeam], ['한 사람', IconMe], ['설정', IconGear],
  ['조절', IconSliders], ['팔레트', IconPalette], ['배치', IconTemplate], ['글꼴', IconType], ['대비', IconContrast],
  ['밝기', IconSun], ['포인터', IconPointer], ['입체', IconBox], ['고치기', IconPencil], ['찾기', IconScan],
  ['새로고침', IconRefresh], ['반짝', IconSpark], ['교통카드', IconTrain],
];
