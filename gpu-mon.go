package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

//var 기준_온도 float64 = 48 // 섭씨

var 기준_온도 float64 = 10

func main() {
	f기준_온도_설정()

	fmt.Println("실행을 중지하려면 'Ctrl+C'를 누르세요.")
	fmt.Printf("\n기준 온도 : %v°C\n\n", int(기준_온도))

	티커 := time.NewTicker(10 * time.Second) // 10초마다 확인.
	ch종료 := make(chan struct{})

	gpu온도_확인()

	go func() {
		for { // Ctrl-C로 중지할 때까지 무한 반복
			select {
			case <-티커.C:
				gpu온도_확인()
			case <-ch종료:
				티커.Stop()
				return
			}
		}
	}()

	// Wait for a signal to ch종료 (e.g. Ctrl+C)
	<-ch종료
}

func f기준_온도_설정() {
	if len(os.Args) > 1 {
		for _, 인수 := range os.Args[1:] {
			if 값, 에러 := strconv.ParseFloat(인수, 64); 에러 == nil {
				기준_온도 = 값
				break
			}
		}
	}
}

// GPU 온도 알아내기.
func gpu온도_측정() ([]float64, error) {
	cli명령 := exec.Command("nvidia-smi", "--query-gpu", "temperature.gpu", "--format=csv,noheader")
	출력_문자열, 에러 := cli명령.Output()
	if 에러 != nil {
		return nil, 에러
	}

	행_모음 := strings.Split(string(출력_문자열), "\n")
	온도_모음 := make([]float64, 0)

	for _, 행 := range 행_모음 {
		if 온도_문자열 := strings.TrimSpace(행); 온도_문자열 == "" {
			break
		} else if 온도, 에러 := strconv.ParseFloat(온도_문자열, 64); 에러 == nil {
			온도_모음 = append(온도_모음, 온도)
		}
	}

	return 온도_모음, nil
}

// 경고음 발생시키기
func f경고음_발생() {
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

func f관리자_여부() bool {
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

	// 아래 행이 왜 동작하는 지는 모르겠지만, 하여튼 정상 동작한다.
	// https://github.com/golang/go/issues/28804#issuecomment-438838144
	token := windows.Token(0)

	if 관리자_여부, 에러 := token.IsMember(sid); 에러 != nil {
		return false
	} else {
		return 관리자_여부
	}
}

var 최저_속도로_낮추기_완료 = false

func f최저_속도로_낮추기() {
	if 최저_속도로_낮추기_완료 {
		return // 중복 실행 방지
	} else {
		최저_속도로_낮추기_완료 = true
	}

	fmt.Printf("** GPU 동작 클럭을 최저로 낮춰서 과열을 방지합니다. **\n")

	if !f관리자_여부() {
		fmt.Printf("** GPU 동작 클럭을 낮추려면 관리자 권한이 필요합니다. **\n")
		f관리자_권한으로_재실행()
	}

	커맨드_클럭_확인 := exec.Command("nvidia-smi", "-q", "-d", "CLOCK")
	출력_바이트_모음, 에러 := 커맨드_클럭_확인.CombinedOutput()
	if 에러 != nil {
		fmt.Println(에러)
		return
	}

	전체_바이트_모음 := bytes.Split(출력_바이트_모음, []byte("\n"))
	행_문자열_모음 := make([]string, 0)
	인덱스_SM_Clock_Samples := 0
	인덱스_Memory_Clock_Samples := 0

	for i, 행_바이트_모음 := range 전체_바이트_모음 {
		행_문자열 := strings.ReplaceAll(string(행_바이트_모음), "\r", "")
		행_문자열_모음 = append(행_문자열_모음, 행_문자열)

		if strings.Contains(행_문자열, "SM Clock Samples") {
			인덱스_SM_Clock_Samples = i
		} else if strings.Contains(행_문자열, "Memory Clock Samples") {
			인덱스_Memory_Clock_Samples = i
		}
	}

	행_GPU := 행_문자열_모음[인덱스_SM_Clock_Samples+4]
	GPU_클럭 := f숫자_추출(행_GPU)

	행_메모리 := 행_문자열_모음[인덱스_Memory_Clock_Samples+4]
	메모리_클럭 := f숫자_추출(행_메모리)

	ac인수 := 메모리_클럭 + "," + GPU_클럭

	fmt.Println(ac인수)

	커맨드_실행 := exec.Command("nvidia-smi", "-ac", ac인수)
	커맨드_실행.Run()
}

func f숫자_추출(문자열 string) string {
	re, _ := regexp.Compile("(\\d+)")
	return re.FindString(문자열)
}

func gpu온도_확인() {
	if 온도_모음, 에러 := gpu온도_측정(); 에러 == nil {
		버퍼 := new(bytes.Buffer)
		경고음_발생_여부 := false

		for i, 온도 := range 온도_모음 {
			버퍼.WriteString(fmt.Sprintf("%.0f°C", 온도))

			if 온도 > 기준_온도 {
				버퍼.WriteString("(!!)")
				경고음_발생_여부 = true
			}

			if i < len(온도_모음)-1 {
				버퍼.WriteString(", ")
			}
		}

		시각_문자열 := time.Now().Format("15:04:05")
		if 경고음_발생_여부 {
			fmt.Printf("** GPU 과열 ** %s : %s\n", 시각_문자열, 버퍼.String())
			f경고음_발생()
			f최저_속도로_낮추기()
		} else {
			fmt.Printf("%s : %s\n", 시각_문자열, 버퍼.String())
		}
	}
}

func f관리자_권한으로_재실행() {
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
