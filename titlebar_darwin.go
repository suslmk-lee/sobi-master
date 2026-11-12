// titlebar_darwin.go
//go:build darwin

package main

// 창 제목 표시줄을 앱 테마에 맞춘다 (macOS).
//
// 제목 표시줄을 투명하게 두고 창 바탕색을 테마의 윗부분 색으로 칠한다(내용은 제목 표시줄 아래에 그대로 있다).
// 제목 글자 · 신호등 버튼의 밝기는 창의 모양(밝게/어둡게)이 정하므로 테마에 맞춰 정한다.
// 단 '시스템' 테마에서는 창 모양을 정하지 않고 macOS 설정을 따르게 둔다 — 창 모양을 정하면 웹뷰의
// prefers-color-scheme 도 그 모양을 따라가, 시스템 테마가 macOS 설정이 바뀐 걸 알아채지 못한다.

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

// appearance: 0 = macOS 설정 따름, 1 = 밝게, 2 = 어둡게
static void sobiSetTitleBar(double r, double g, double b, int appearance) {
	dispatch_async(dispatch_get_main_queue(), ^{
		NSAppearance *ap = nil;
		if (appearance == 1) ap = [NSAppearance appearanceNamed:NSAppearanceNameAqua];
		if (appearance == 2) ap = [NSAppearance appearanceNamed:NSAppearanceNameDarkAqua];
		NSColor *bg = [NSColor colorWithSRGBRed:r green:g blue:b alpha:1.0];
		for (NSWindow *w in [NSApp windows]) {
			// 앱 창만 (대화상자 · 패널 · 제목 표시줄 없는 창은 건드리지 않는다)
			if ([w isKindOfClass:[NSPanel class]] || !([w styleMask] & NSWindowStyleMaskTitled)) continue;
			[w setTitlebarAppearsTransparent:YES];
			[w setBackgroundColor:bg];
			[w setAppearance:ap];
			if (@available(macOS 11.0, *)) {
				[w setTitlebarSeparatorStyle:NSTitlebarSeparatorStyleNone];
			}
		}
	});
}
*/
import "C"

import (
	"fmt"
	"strconv"
	"strings"
)

// parseHexRGB 는 "#rrggbb" 를 0~1 범위 RGB 로 읽는다.
func parseHexRGB(s string) (r, g, b float64, err error) {
	h := strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(h) != 6 {
		return 0, 0, 0, fmt.Errorf("색 형식이 아닙니다: %q", s)
	}
	n, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("색 형식이 아닙니다: %q", s)
	}
	return float64(n>>16&0xff) / 255, float64(n>>8&0xff) / 255, float64(n&0xff) / 255, nil
}

// SetTitleBarColors 는 제목 표시줄 바탕색과 밝게/어둡게를 맞춘다 (프론트엔드가 테마가 바뀔 때 부른다).
// text(글자색)는 macOS 에서는 창 모양이 정하므로 쓰지 않는다. followSystem 이면 창 모양을 macOS 설정에 맡긴다.
func (a *App) SetTitleBarColors(background, text string, dark, followSystem bool) error {
	r, g, b, err := parseHexRGB(background)
	if err != nil {
		return err
	}
	appearance := 1
	switch {
	case followSystem:
		appearance = 0
	case dark:
		appearance = 2
	}
	C.sobiSetTitleBar(C.double(r), C.double(g), C.double(b), C.int(appearance))
	return nil
}
