//go:build windows

package gpu_mon

import (
	"fmt"
	"golang.org/x/sys/windows"
	"log"
	"os"
	"strings"
	"syscall"
)

// F경고음_발생 :  경고음 발생시키기
func F경고음_발생() {
	dll, 에러 := syscall.LoadDLL("user32.dll")
	if 에러 != nil {
		fmt.Println(에러)
		return
	}
	defer dll.Release()

	proc, 에러 := dll.FindProc("MessageBeep")
	if 에러 != nil {
		fmt.Println(에러)
		return
	}

	반환값, _, _ := proc.Call(0xFFFFFFFF) // '0xFFFFFFFF'은 경고음 표준 주파수.
	if 반환값 == 0 {
		fmt.Println("경고음 발생 실패.")
	}
}

// 참고 링크 : https://github.com/golang/go/issues/28804#issuecomment-505326268
// F관리자_여부 : 관리자 권한 여부 확인
func F관리자_여부() bool {
	var sid *windows.SID

	// MS의 공식 안내를 Go언어로 포팅.
	// MS의 C++ 공식 문서는 다음 링크를 참조한다.
	// https://docs.microsoft.com/en-us/windows/desktop/api/securitybaseapi/nf-securitybaseapi-checktokenmembership
	에러 := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid)
	if 에러 != nil {
		log.Fatalf("SID Error: %s", 에러)
		return false
	}
	defer windows.FreeSid(sid)

	token := windows.Token(0)

	if 관리자_여부, 에러 := token.IsMember(sid); 에러 != nil {
		return false
	} else {
		return 관리자_여부
	}
}

// F관리자_권한으로_재실행 : 관리자 권한으로 재실행
func F관리자_권한으로_재실행() {
	verb := "runas"
	exe, _ := os.Executable()
	cwd, _ := os.Getwd()
	args := strings.Join(os.Args[1:], " ")

	verbPtr, _ := syscall.UTF16PtrFromString(verb)
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	cwdPtr, _ := syscall.UTF16PtrFromString(cwd)
	argPtr, _ := syscall.UTF16PtrFromString(args)

	var showCmd int32 = 1 //SW_NORMAL

	err := windows.ShellExecute(0, verbPtr, exePtr, argPtr, cwdPtr, showCmd)
	if err != nil {
		fmt.Println(err)
	}
}
