// frontend/src/components/effects/CardTilt.tsx
import { useEffect } from 'react';
import { useAppearance } from '../../appearance/AppearanceContext';

// 카드 입체감: 마우스를 올린 카드를 포인터 쪽으로 기울이고 살짝 띄운다 (모양 설정에서 켤 때만)
const BASE_WIDTH = 480; // 이보다 넓은 카드는 기울기를 줄여 과하지 않게
const BASE_HEIGHT = 320; // 이보다 높은 카드는 앞뒤 기울기를 줄인다
const MAX_TILT_HEIGHT = 900; // 이보다 높은 카드는 기울이지 않는다 (띄우기만)

export default function CardTilt() {
  const { settings } = useAppearance();
  const { enabled, scope, tilt, lift } = settings.cardDepth;
  const active = enabled && (tilt > 0 || lift > 0);

  useEffect(() => {
    if (!active) return;
    if (window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return;
    const selector = scope === 'all' ? '.surface, .hero-grain' : '.hero-grain';
    let current: HTMLElement | null = null;
    let frame = 0;
    let last: PointerEvent | null = null;

    const release = (el: HTMLElement) => {
      el.style.transition = 'transform 0.35s cubic-bezier(0.22, 1, 0.36, 1)';
      el.style.transform = '';
    };

    const apply = () => {
      frame = 0;
      const e = last;
      if (!e) return;
      const target = e.target instanceof Element ? (e.target.closest(selector) as HTMLElement | null) : null;
      // 모달·알림 드롭다운처럼 화면에 고정된 창은 움직이지 않는다
      const el =
        target && (target === current || (!target.closest('[role=dialog], .fixed') && getComputedStyle(target).position !== 'fixed'))
          ? target
          : null;
      if (current && current !== el) release(current);
      current = el;
      if (!el) return;
      const r = el.getBoundingClientRect();
      // 화면보다 긴 카드(거래 표 등)는 기울이지 않고 띄우기만 한다. 수천 px 짜리 판을 원근으로 기울이면
      // 끝부분이 시점 앞으로 넘어가 브라우저가 판 전체를 그리지 않아 깜빡이거나 사라진다.
      // data-no-tilt 를 단 영역도 기울이지 않는다.
      if (r.height > MAX_TILT_HEIGHT || el.closest('[data-no-tilt]')) {
        el.style.transition = 'transform 0.12s ease-out';
        el.style.transform = lift > 0 ? `translateY(${-lift}px)` : '';
        return;
      }
      const px = (e.clientX - r.left) / r.width - 0.5;
      const py = (e.clientY - r.top) / r.height - 0.5;
      // 넓은 카드는 좌우 기울기를, 높은 카드는 앞뒤 기울기를 줄인다 (끝이 튀어나오는 정도를 일정하게)
      const kx = Math.min(1, BASE_WIDTH / r.width);
      const ky = Math.min(1, BASE_HEIGHT / r.height);
      el.style.transition = 'transform 0.12s ease-out';
      el.style.transform = `perspective(1000px) rotateX(${(-py * 2 * tilt * ky).toFixed(2)}deg) rotateY(${(px * 2 * tilt * kx).toFixed(2)}deg) translateY(${-lift}px)`;
    };

    const onMove = (e: PointerEvent) => {
      if (e.pointerType === 'touch') return;
      last = e;
      if (!frame) frame = requestAnimationFrame(apply);
    };
    const onLeave = () => {
      if (current) release(current);
      current = null;
    };

    window.addEventListener('pointermove', onMove, { passive: true });
    document.documentElement.addEventListener('mouseleave', onLeave);
    window.addEventListener('blur', onLeave);
    return () => {
      window.removeEventListener('pointermove', onMove);
      document.documentElement.removeEventListener('mouseleave', onLeave);
      window.removeEventListener('blur', onLeave);
      if (frame) cancelAnimationFrame(frame);
      onLeave();
    };
  }, [active, scope, tilt, lift]);

  return null;
}
