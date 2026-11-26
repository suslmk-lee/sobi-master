//go:build windows

package main

import (
	"fmt"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

// exeVersionInfo 는 exe 의 버전 리소스에서 읽은 값 (wails build 가 build/windows/info.json 으로 넣는다).
type exeVersionInfo struct {
	ProductVersion string // 문자열 버전 정보 → 없으면 고정 파일 버전(주.부.수)
	ProductName    string
}

// readExeVersionInfo 는 exe 의 버전 리소스를 읽는다. 리소스가 없으면 오류.
func readExeVersionInfo(path string) (exeVersionInfo, error) {
	var zero windows.Handle
	size, err := windows.GetFileVersionInfoSize(path, &zero)
	if err != nil || size == 0 {
		return exeVersionInfo{}, fmt.Errorf("%s 에 버전 정보가 없습니다", filepath.Base(path))
	}
	buf := make([]byte, size)
	if err := windows.GetFileVersionInfo(path, 0, size, unsafe.Pointer(&buf[0])); err != nil {
		return exeVersionInfo{}, fmt.Errorf("%s 의 버전 정보를 읽지 못했습니다: %w", filepath.Base(path), err)
	}
	block := unsafe.Pointer(&buf[0])

	query := func(sub string) (unsafe.Pointer, uint32, bool) {
		var p unsafe.Pointer
		var n uint32
		if windows.VerQueryValue(block, sub, unsafe.Pointer(&p), &n) != nil || p == nil || n == 0 {
			return nil, 0, false
		}
		return p, n, true
	}
	str := func(sub string) (string, bool) {
		p, n, ok := query(sub)
		if !ok {
			return "", false
		}
		// n 은 문자 수(끝의 NUL 포함)
		s := windows.UTF16ToString(unsafe.Slice((*uint16)(p), n))
		return s, s != ""
	}

	// 언어·코드 페이지 후보: 파일에 적힌 번역 목록 → 흔한 값
	var langs []string
	if p, n, ok := query(`\VarFileInfo\Translation`); ok {
		for i := uint32(0); i+4 <= n; i += 4 {
			t := unsafe.Slice((*uint16)(unsafe.Add(p, i)), 2)
			langs = append(langs, fmt.Sprintf("%04x%04x", t[0], t[1]))
		}
	}
	langs = append(langs, "000004b0", "040904b0", "040904e4")

	var info exeVersionInfo
	for _, l := range langs {
		if info.ProductVersion == "" {
			info.ProductVersion, _ = str(`\StringFileInfo\` + l + `\ProductVersion`)
		}
		if info.ProductName == "" {
			info.ProductName, _ = str(`\StringFileInfo\` + l + `\ProductName`)
		}
	}
	if info.ProductVersion == "" {
		if p, n, ok := query(`\`); ok && n >= uint32(unsafe.Sizeof(windows.VS_FIXEDFILEINFO{})) {
			ff := (*windows.VS_FIXEDFILEINFO)(p)
			info.ProductVersion = fmt.Sprintf("%d.%d.%d", ff.FileVersionMS>>16, ff.FileVersionMS&0xffff, ff.FileVersionLS>>16)
		}
	}
	if info.ProductVersion == "" {
		return exeVersionInfo{}, fmt.Errorf("%s 에 버전 정보가 없습니다", filepath.Base(path))
	}
	return info, nil
}

// verifyStagedExecutable 은 풀어 둔 새 exe 가 받으려던 버전이고, 지금 앱과 같은 제품인지 확인한다.
func verifyStagedExecutable(staged, current, wantVersion string) error {
	got, err := readExeVersionInfo(staged)
	if err != nil {
		return err
	}
	if compareVersions(got.ProductVersion, wantVersion) != 0 {
		return fmt.Errorf("설치 파일의 버전(%s)이 받으려던 버전(%s)과 다릅니다", got.ProductVersion, wantVersion)
	}
	if cur, err := readExeVersionInfo(current); err == nil && cur.ProductName != "" && got.ProductName != cur.ProductName {
		return fmt.Errorf("설치 파일의 앱이 이 앱과 다릅니다 (%s)", got.ProductName)
	}
	return nil
}
