// frontend/src/appearance/AppearanceContext.tsx
import { createContext, ReactNode, useCallback, useContext, useEffect, useMemo, useState, CSSProperties } from 'react';
import { buildThemeVars, getTheme, hexToRgb, mix, prefersDark, rgbToHex, DEFAULT_THEME_ID, SYSTEM_THEME_ID, AppTheme } from './themes';
import { desktop } from '../platform/desktop';
// 데스크톱 셸이 아니면 제목 표시줄 브리지(setTitleBarColors)와 아래 useEffect 를 삭제
import {
  DEFAULT_CARD_STYLE, DEFAULT_FONT_SIZE, DEFAULT_MONO, DEFAULT_SANS, FONT_SIZES, FontSizeId,
  MONO_FALLBACK, MONO_FONTS, SANS_FALLBACK, SANS_FONTS, CARD_STYLES, CURSOR_TRAIL_LIMITS,
  CARD_GLOW_LIMITS, CARD_DEPTH_LIMITS, POINTER_PRESETS, TRAIL_PRESETS, type PointerPresetId, type TrailPresetId,
} from './options';

// 커서를 따라다니는 점 체인 효과 (장식용, 기본 꺼짐)
export interface CursorTrailSettings {
  enabled: boolean;
  nodes: number; // 점 개수
  ease: number; // 앞 점을 따라가는 빠르기 (클수록 빠름)
  headSize: number; // 머리 점 지름(px)
  taper: number; // 꼬리로 갈수록 작아지는 정도 (0~1)
  gradient: boolean; // 체인을 따라 mint → sky 그라디언트
  style: TrailPresetId; // 트레일 프리셋 (수치 설정은 모든 프리셋이 공유)
}

export const DEFAULT_CURSOR_TRAIL: CursorTrailSettings = {
  enabled: false, nodes: 14, ease: 0.34, headSize: 24, taper: 0.8, gradient: false, style: 'dots',
};

// 마우스 포인터 자체를 바꾸는 프리셋 ('native' = 운영체제 커서)
export interface CursorPointerSettings {
  style: PointerPresetId;
}

export const DEFAULT_CURSOR_POINTER: CursorPointerSettings = { style: 'native' };

// 카드에 덧입히는 원형 광원 그라데이션 (기본 꺼짐 → 카드 스타일 본래 모양)
export interface CardGlowSettings {
  enabled: boolean;
  scope: 'hero' | 'all'; // 대표 카드(hero)만 / 모든 카드
  intensity: number; // 0~0.5
  size: number; // 광원 반경(%)
  x: number; // 광원 중심 가로(%)
  y: number; // 광원 중심 세로(%)
  gradient: boolean; // 강조색 → 하늘색 두 광원
}

export const DEFAULT_CARD_GLOW: CardGlowSettings = {
  enabled: false, scope: 'hero', intensity: 0.18, size: 65, x: 0, y: 0, gradient: false,
};

// 카드 입체감: 그림자 깊이·모서리 빛(베벨)·마우스 기울기·떠오름 (기본 꺼짐)
export interface CardDepthSettings {
  enabled: boolean;
  scope: 'hero' | 'all';
  depth: number; // 바깥 그림자 0~1
  bevel: number; // 위 모서리 빛 / 아래 모서리 그늘 0~1
  tilt: number; // 마우스를 올렸을 때 최대 기울기(도), 0 = 끔
  lift: number; // 마우스를 올렸을 때 떠오르는 높이(px)
}

export const DEFAULT_CARD_DEPTH: CardDepthSettings = {
  enabled: false, scope: 'all', depth: 0.5, bevel: 0.5, tilt: 5, lift: 3,
};

export interface AppearanceSettings {
  themeId: string;
  cardStyle: string;
  sansFont: string;
  monoFont: string;
  fontSize: FontSizeId;
  cursorTrail: CursorTrailSettings;
  cursorPointer: CursorPointerSettings;
  cardGlow: CardGlowSettings;
  cardDepth: CardDepthSettings;
}

export const DEFAULT_APPEARANCE: AppearanceSettings = {
  themeId: DEFAULT_THEME_ID,
  cardStyle: DEFAULT_CARD_STYLE,
  sansFont: DEFAULT_SANS,
  monoFont: DEFAULT_MONO,
  fontSize: DEFAULT_FONT_SIZE,
  cursorTrail: DEFAULT_CURSOR_TRAIL,
  cursorPointer: DEFAULT_CURSOR_POINTER,
  cardGlow: DEFAULT_CARD_GLOW,
  cardDepth: DEFAULT_CARD_DEPTH,
};

// index.html 의 첫 페인트 전 스크립트도 이 키를 읽는다 (깜빡임 방지)
const STORAGE_KEY = 'app-appearance';

// 제목 표시줄 색 맞추기
const AMBIENT_TOP = 0.06; // App 셸 위쪽 광원(radial-gradient white 6%)의 맨 위 세기
const TITLEBAR_RETRY_MS = [250, 600, 1200, 2500, 5000]; // 창이 뜨기 전 실패 시 재시도 간격

// 헤더 바탕(--header-bg)의 불투명도. "rgba(6, 6, 7, 0.7)" / "rgb(6 6 7 / 0.7)" → 0.7, 알파 없으면 1.
// 헤더가 아직 없거나 읽지 못하면 기본값 0.7.
function headerAlpha(): number {
  const el = document.querySelector('.app-header');
  if (!el) return 0.7;
  const bg = getComputedStyle(el).backgroundColor;
  const m = /rgba?\(([^)]*)\)/.exec(bg);
  if (!m) return 0.7;
  const parts = m[1].split(/[\s,/]+/).filter(Boolean);
  if (parts.length < 4) return 1;
  const a = Number(parts[3]);
  return Number.isFinite(a) ? a : 0.7;
}

const clampNum = (v: unknown, lim: { min: number; max: number }, def: number) =>
  typeof v === 'number' && Number.isFinite(v) ? Math.min(lim.max, Math.max(lim.min, v)) : def;

// 저장값(없거나 깨졌을 수 있음)을 기본값과 합치고 범위로 보정
function sanitizeCursorTrail(raw: unknown): CursorTrailSettings {
  const d = DEFAULT_CURSOR_TRAIL;
  const r = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const L = CURSOR_TRAIL_LIMITS;
  return {
    enabled: typeof r.enabled === 'boolean' ? r.enabled : d.enabled,
    nodes: Math.round(clampNum(r.nodes, L.nodes, d.nodes)),
    ease: clampNum(r.ease, L.ease, d.ease),
    headSize: clampNum(r.headSize, L.headSize, d.headSize),
    taper: clampNum(r.taper, L.taper, d.taper),
    gradient: typeof r.gradient === 'boolean' ? r.gradient : d.gradient,
    style: TRAIL_PRESETS.some((p) => p.id === r.style) ? (r.style as TrailPresetId) : d.style,
  };
}

function sanitizeCursorPointer(raw: unknown): CursorPointerSettings {
  const r = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    style: POINTER_PRESETS.some((p) => p.id === r.style) ? (r.style as PointerPresetId) : DEFAULT_CURSOR_POINTER.style,
  };
}

function sanitizeCardGlow(raw: unknown): CardGlowSettings {
  const d = DEFAULT_CARD_GLOW;
  const r = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const L = CARD_GLOW_LIMITS;
  return {
    enabled: typeof r.enabled === 'boolean' ? r.enabled : d.enabled,
    scope: r.scope === 'all' || r.scope === 'hero' ? r.scope : d.scope,
    intensity: clampNum(r.intensity, L.intensity, d.intensity),
    size: clampNum(r.size, L.size, d.size),
    x: clampNum(r.x, L.x, d.x),
    y: clampNum(r.y, L.y, d.y),
    gradient: typeof r.gradient === 'boolean' ? r.gradient : d.gradient,
  };
}

function sanitizeCardDepth(raw: unknown): CardDepthSettings {
  const d = DEFAULT_CARD_DEPTH;
  const r = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const L = CARD_DEPTH_LIMITS;
  return {
    enabled: typeof r.enabled === 'boolean' ? r.enabled : d.enabled,
    scope: r.scope === 'all' || r.scope === 'hero' ? r.scope : d.scope,
    depth: clampNum(r.depth, L.depth, d.depth),
    bevel: clampNum(r.bevel, L.bevel, d.bevel),
    tilt: clampNum(r.tilt, L.tilt, d.tilt),
    lift: clampNum(r.lift, L.lift, d.lift),
  };
}

// 카드 본래 그림자 앞에 덧붙일 입체 그림자 (.surface / .hero-grain 이 --fx-depth-* 를 읽는다)
function cardDepthShadow(c: CardDepthSettings) {
  const { depth: d, bevel: b } = c;
  const f = (v: number) => v.toFixed(2);
  const parts: string[] = [];
  if (d > 0) {
    parts.push(`0 ${f(1 + 2 * d)}px ${f(2 + 4 * d)}px rgb(var(--c-shadow) / ${f(0.3 * d)})`);
    parts.push(`0 ${f(8 + 20 * d)}px ${f(16 + 40 * d)}px -${f(6 + 8 * d)}px rgb(var(--c-shadow) / ${f(0.75 * d)})`);
  }
  if (b > 0) {
    // 위쪽은 밝게, 아래쪽은 어둡게 → 돌출된 느낌 (라이트 테마에서도 같은 흰 빛)
    parts.push(`inset 0 1px 0 rgb(255 255 255 / ${f(0.16 * b)})`);
    parts.push(`inset 0 -${f(1 + 2 * b)}px ${f(2 + 3 * b)}px rgb(var(--c-shadow) / ${f(0.4 * b)})`);
  }
  return parts.length ? parts.join(', ') : '0 0 #0000';
}

// 카드 배경 맨 위에 겹칠 광원 레이어 (.surface / .hero-grain 이 --fx-glow-* 를 읽는다)
function cardGlowLayer(g: CardGlowSettings) {
  const a = g.intensity.toFixed(3);
  const main = `radial-gradient(circle at ${g.x}% ${g.y}%, rgb(var(--c-mint-400) / ${a}) 0%, transparent ${g.size}%)`;
  if (!g.gradient) return main;
  // 두 번째 광원은 반대쪽 모서리에서 하늘색으로
  const sub = `radial-gradient(circle at ${100 - g.x}% ${100 - g.y}%, rgb(var(--c-sky-400) / ${(g.intensity * 0.8).toFixed(3)}) 0%, transparent ${g.size}%)`;
  return `${main}, ${sub}`;
}

function loadSettings(): AppearanceSettings {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return DEFAULT_APPEARANCE;
    const saved = JSON.parse(raw)?.settings ?? {};
    const merged = { ...DEFAULT_APPEARANCE, ...saved } as AppearanceSettings;
    // 사라진 선택지가 저장돼 있으면 기본값으로
    if (!CARD_STYLES.some((c) => c.id === merged.cardStyle)) merged.cardStyle = DEFAULT_CARD_STYLE;
    if (!SANS_FONTS.some((f) => f.id === merged.sansFont)) merged.sansFont = DEFAULT_SANS;
    if (!MONO_FONTS.some((f) => f.id === merged.monoFont)) merged.monoFont = DEFAULT_MONO;
    if (!FONT_SIZES.some((f) => f.id === merged.fontSize)) merged.fontSize = DEFAULT_FONT_SIZE;
    merged.cursorTrail = sanitizeCursorTrail(saved.cursorTrail);
    merged.cursorPointer = sanitizeCursorPointer(saved.cursorPointer);
    merged.cardGlow = sanitizeCardGlow(saved.cardGlow);
    merged.cardDepth = sanitizeCardDepth(saved.cardDepth);
    return merged;
  } catch {
    return DEFAULT_APPEARANCE;
  }
}

// 설정 → <html> 에 적용할 CSS 변수/속성
export function computeRootStyle(s: AppearanceSettings, dark: boolean = prefersDark()) {
  const theme = getTheme(s.themeId, dark);
  const vars: Record<string, string> = { ...buildThemeVars(theme) };
  const sans = SANS_FONTS.find((f) => f.id === s.sansFont) ?? SANS_FONTS[0];
  const mono = MONO_FONTS.find((f) => f.id === s.monoFont) ?? MONO_FONTS[0];
  vars['--font-sans'] = `${sans.family}, ${SANS_FALLBACK}`;
  vars['--font-mono'] = `${mono.family}, ${MONO_FALLBACK}`;
  vars['font-size'] = `${FONT_SIZES.find((f) => f.id === s.fontSize)?.px ?? 16}px`;
  const glow = s.cardGlow?.enabled ? cardGlowLayer(s.cardGlow) : 'none';
  vars['--fx-glow-hero'] = glow;
  vars['--fx-glow-card'] = s.cardGlow?.enabled && s.cardGlow.scope === 'all' ? glow : 'none';
  const depth = s.cardDepth?.enabled ? cardDepthShadow(s.cardDepth) : '0 0 #0000';
  vars['--fx-depth-hero'] = depth;
  vars['--fx-depth-card'] = s.cardDepth?.enabled && s.cardDepth.scope === 'all' ? depth : '0 0 #0000';
  return { vars, attrs: { 'data-card': s.cardStyle, 'data-theme-mode': theme.mode, 'data-theme': theme.id } };
}

function applyToRoot(s: AppearanceSettings, dark: boolean = prefersDark()) {
  const root = document.documentElement;
  const { vars, attrs } = computeRootStyle(s, dark);
  Object.entries(vars).forEach(([k, v]) => root.style.setProperty(k, v));
  Object.entries(attrs).forEach(([k, v]) => root.setAttribute(k, v));
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ settings: s, vars, attrs }));
  } catch {
    // 저장 불가(사생활 모드 등)여도 화면 적용은 유지
  }
}

// main.tsx 에서 첫 렌더 전에 호출 → 저장된 테마로 바로 그려진다
export function initAppearance() {
  applyToRoot(loadSettings());
}

interface AppearanceContextValue {
  settings: AppearanceSettings;
  theme: AppTheme;
  update: (patch: Partial<AppearanceSettings>) => void;
  reset: () => void;
}

const AppearanceContext = createContext<AppearanceContextValue | null>(null);

export function AppearanceProvider({ children }: { children: ReactNode }) {
  const [settings, setSettings] = useState<AppearanceSettings>(loadSettings);
  // '시스템' 테마일 때 따라갈 macOS 밝게/어둡게 설정 — 앱이 켜진 채로 바뀌어도 바로 따라간다
  const [osDark, setOsDark] = useState(prefersDark);
  const theme = useMemo(() => getTheme(settings.themeId, osDark), [settings.themeId, osDark]);
  const followSystem = settings.themeId === SYSTEM_THEME_ID;

  useEffect(() => {
    if (settings.themeId !== SYSTEM_THEME_ID || !window.matchMedia) return;
    const mq = window.matchMedia('(prefers-color-scheme: dark)');
    const on = () => setOsDark(mq.matches);
    on();
    mq.addEventListener('change', on);
    return () => mq.removeEventListener('change', on);
  }, [settings.themeId]);

  useEffect(() => {
    applyToRoot(settings, osDark);
  }, [settings, osDark]);

  // 데스크톱 창 제목 표시줄을 앱 윗부분과 같은 색으로 맞춘다.
  // '시스템' 테마면 macOS 창 모양을 macOS 설정에 맡긴다(정해 버리면 웹뷰가 macOS 설정 변화를 못 알아챈다).
  // 앱 윗부분은 배경(ink-950) 위에 위쪽 은은한 빛(white 6%)과 반투명 헤더(--header-bg)가 겹쳐 있으므로
  // 그 합성색을 계산해 보낸다. 시작 직후엔 창이 아직 보이지 않아 실패할 수 있어 몇 번 다시 시도하고,
  // 창이 다시 활성화될 때도 한 번 더 맞춘다.
  useEffect(() => {
    const send = () => {
      const css = getComputedStyle(document.documentElement);
      const triplet = (name: string) => css.getPropertyValue(name).trim().split(/\s+/).map(Number);
      const toHex = (v: number[], fallback: string) =>
        v.length === 3 && v.every(Number.isFinite) ? rgbToHex(v as [number, number, number]) : fallback;
      const base = toHex(triplet('--c-ink-950'), theme.bg);
      const white = toHex(triplet('--c-white'), theme.fg);
      const text = toHex(triplet('--c-zinc-300'), theme.fg);
      const top = mix(base, white, AMBIENT_TOP * (1 - headerAlpha()));
      return desktop.setTitleBarColors(top, text, theme.mode === 'dark', followSystem); // §10.0 브리지 — 웹이면 아무것도 안 함
    };
    let cancelled = false;
    const timers: number[] = [];
    // 즉시 시도하고, 실패하면(창이 아직 안 뜸) 간격을 늘려 가며 다시 시도한다
    const attempt = (i: number) => {
      if (cancelled) return;
      send().then((ok) => {
        if (!ok && !cancelled && i < TITLEBAR_RETRY_MS.length) {
          timers.push(window.setTimeout(() => attempt(i + 1), TITLEBAR_RETRY_MS[i]));
        }
      });
    };
    // applyToRoot 가 바꾼 변수가 반영된 다음 프레임에 계산한다
    const raf = requestAnimationFrame(() => attempt(0));
    const onFocus = () => void send();
    window.addEventListener('focus', onFocus);
    return () => {
      cancelled = true;
      cancelAnimationFrame(raf);
      timers.forEach((t) => window.clearTimeout(t));
      window.removeEventListener('focus', onFocus);
    };
  }, [theme, followSystem, settings.cardStyle]);

  const update = useCallback((patch: Partial<AppearanceSettings>) => setSettings((prev) => ({ ...prev, ...patch })), []);
  const reset = useCallback(() => setSettings(DEFAULT_APPEARANCE), []);

  const value = useMemo(
    () => ({ settings, theme, update, reset }),
    [settings, theme, update, reset],
  );
  return <AppearanceContext.Provider value={value}>{children}</AppearanceContext.Provider>;
}

export function useAppearance() {
  const ctx = useContext(AppearanceContext);
  if (!ctx) throw new Error('useAppearance must be used within AppearanceProvider');
  return ctx;
}

export interface ThemeColors {
  mode: 'dark' | 'light';
  accent: string; // 강조 (차트 주 계열)
  accentSoft: string;
  restricted: string; // 보조 상태 계열 (앰버)
  positive: string;
  negative: string;
  text: string; // 최대 대비 글자
  muted: string; // 보조 글자
  axis: string; // 차트 축 글자
  grid: string; // 차트 격자/트랙
  surface: string;
  palette: string[]; // 파이·범주형 차트용
  tooltip: { contentStyle: CSSProperties; labelStyle: CSSProperties; itemStyle: CSSProperties };
}

// 앞에서부터 고르되, 이미 고른 색과 너무 비슷한 색은 건너뛴다 (강조색이 상태색과 겹치는 테마 대비)
function distinctColors(colors: string[], minDist = 70): string[] {
  const picked: string[] = [];
  for (const col of colors) {
    const [r, g, b] = hexToRgb(col);
    const close = picked.some((p) => {
      const [pr, pg, pb] = hexToRgb(p);
      return Math.hypot(r - pr, g - pg, b - pb) < minDist;
    });
    if (!close) picked.push(col);
  }
  return picked;
}

const hexOf = (vars: Record<string, string>, name: string) =>
  rgbToHex(vars[`--c-${name}`].split(' ').map(Number) as [number, number, number]);

// Recharts 처럼 CSS 클래스를 못 쓰는 곳(SVG 속성, 인라인 스타일)에서 현재 테마 색을 쓰기 위한 훅
export function useThemeColors(): ThemeColors {
  const { theme } = useAppearance();
  return useMemo(() => {
    const v = buildThemeVars(theme);
    const c = (n: string) => hexOf(v, n);
    const surface = c('surface');
    const border = theme.mode === 'dark' ? 'rgba(255,255,255,0.1)' : 'rgba(0,0,0,0.1)';
    return {
      mode: theme.mode,
      accent: c('mint-400'),
      accentSoft: c('mint-300'),
      restricted: c('amber-400'),
      positive: c('mint-400'),
      negative: c('rose-400'),
      text: c('white'),
      muted: c('zinc-400'),
      axis: c('zinc-500'),
      grid: c('ink-600'),
      surface,
      palette: distinctColors([c('mint-400'), c('sky-400'), c('amber-400'), c('rose-400'), c('orange-400'), c('emerald-400'), c('mint-200'), c('sky-200'), c('amber-200'), c('rose-200'), c('zinc-400')]),
      tooltip: {
        contentStyle: { background: c('ink-850'), border: `1px solid ${border}`, borderRadius: 12, fontSize: 12, color: c('zinc-100') },
        labelStyle: { color: c('zinc-300') },
        itemStyle: { color: c('zinc-100') },
      },
    };
  }, [theme]);
}
