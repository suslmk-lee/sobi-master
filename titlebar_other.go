// titlebar_other.go
//go:build !windows && !darwin

package main

// SetTitleBarColors 는 Windows · macOS 에서만 동작한다 (다른 OS 에서는 아무것도 하지 않음).
func (a *App) SetTitleBarColors(background, text string, dark, followSystem bool) error {
	return nil
}
