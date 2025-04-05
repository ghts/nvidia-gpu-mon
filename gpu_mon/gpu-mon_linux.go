//go:build linux

package gpu_mon

import (
	"fmt"
	"os"
	"os/exec"
)

// F경고음_발생 :  경고음 발생시키기
func F경고음_발생() {
	cmd := exec.Command("paplay", "/usr/share/sounds/freedesktop/stereo/bell.oga")
	if err := cmd.Run(); err != nil {
		fmt.Println("경고음 발생 실패:", err)
	}
}

// F관리자_여부 : 관리자 권한 여부 확인
func F관리자_여부() bool {
	return os.Geteuid() == 0
}

// F관리자_권한으로_재실행 : 관리자 권한으로 재실행
func F관리자_권한으로_재실행() {
	// 현재 실행 중인 실행 파일의 경로를 얻습니다.
	실행_파일_경로, 에러 := os.Executable()
	if 에러 != nil {
		panic("실행 파일 경로 획득 실패.")
	}

	args := append([]string{실행_파일_경로}, os.Args...)

	cmd := exec.Command("sudo", args...)
	cmd.Env = os.Environ()
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Println("관리자 권한으로 재실행 실패:", err)
	}
}
