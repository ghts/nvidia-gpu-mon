package main

import (
	"bytes"
	"fmt"
	"golang.org/x/exp/constraints"
	"golang.org/x/sys/windows"
	"log"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	기준_온도_기본값 = 48.0
	클럭_회복_기준  = 6.0

	대기_시간_일반    = 10 * time.Second
	대기_시간_주의_요망 = 1 * time.Second
)

var (
	Ch종료       = make(chan struct{})
	과열_발생_클럭   float64
	메모리_클럭     float64
	지원되는_클럭_모음 []float64
)

func main() {
	fmt.Println("실행을 중지하려면 'Ctrl+C'를 누르세요.")
	fmt.Printf("\n기준 온도 : %v°C, 현재 클럭 : %vMHz\n\n", int(f기준_온도()), int(f현재_클럭()))

	// 과열_발생_클럭 초기값 설정.
	if 현재_클럭 := f현재_클럭(); 현재_클럭 > f최저_클럭() {
		과열_발생_클럭 = 현재_클럭
	} else {
		과열_발생_클럭 = f최고_클럭()
	}

	// 초기값 설정.
	최근_온도 := 0.0
	주의_요망 := false
	최근_경고_발생_시각 := time.Time{}

	go func() {
		for {
			최근_온도, 주의_요망 = gpu온도_확인(최근_온도)

			// 종료 신호 발생 여부 확인.
			select {
			case <-Ch종료:
				return
			default:
			}

			// 최근_경고_발생_시각 업데이트
			if 주의_요망 {
				최근_경고_발생_시각 = time.Now()
			}

			if f주의_요망(최근_경고_발생_시각) {
				time.Sleep(대기_시간_주의_요망)
			} else {
				time.Sleep(대기_시간_일반)
			}
		}
	}()

	// Wait for a signal to ch종료 (e.g. Ctrl+C)
	<-Ch종료
}

// f주의_요망 : 최근 경고 발생 후 10초 경과하기 전까지 주의 요망 상태 유지.
func f주의_요망(최근_경고_발생_시각 time.Time) bool {
	return time.Now().Before(최근_경고_발생_시각.Add(10 * time.Second))
}

func f기준_온도() float64 {
	기준_온도 := 기준_온도_기본값

	if len(os.Args) > 1 {
		for _, 인수 := range os.Args[1:] {
			if 값, 에러 := strconv.ParseFloat(인수, 64); 에러 == nil {
				기준_온도 = 값
				break
			}
		}
	}

	return 기준_온도
}

// GPU 온도 알아내기.
func gpu온도_측정() ([]float64, error) {
	커맨드 := exec.Command("nvidia-smi", "--query-gpu", "temperature.gpu", "--format=csv,noheader")
	출력_바이트_모음, 에러 := 커맨드.Output()
	if 에러 != nil {
		return nil, 에러
	}

	행_모음 := strings.Split(string(출력_바이트_모음), "\n")
	온도_모음 := make([]float64, 0)

	for _, 행 := range 행_모음 {
		if 온도, 에러 := strconv.ParseFloat(f숫자_추출(행), 64); 에러 == nil {
			온도_모음 = append(온도_모음, 온도)
		}
	}

	return 온도_모음, nil
}

func gpu온도_확인(최근_온도 float64) (현재_온도 float64, 주의_요망 bool) {
	기준_온도 := f기준_온도()
	주의_요망 = false

	if 온도_모음, 에러 := gpu온도_측정(); 에러 == nil {
		현재_온도 = f평균값(온도_모음...)
		최고_온도 := f최대값(온도_모음...)

		if 최고_온도-현재_온도 > 2.0 {
			현재_온도 = 최고_온도 // GPU 1개만 집중적으로 사용되는 경우 대응.
		}

		기준_온도_초과 := false
		버퍼 := new(bytes.Buffer)

		for i, 온도 := range 온도_모음 {
			버퍼.WriteString(fmt.Sprintf("%.0f°C", 온도))

			if 온도 > 기준_온도 {
				버퍼.WriteString("(!!)")
				기준_온도_초과 = true
			}

			if i < len(온도_모음)-1 {
				버퍼.WriteString(", ")
			}
		}

		온도_상승치 := 현재_온도 - 최근_온도
		if 최근_온도 == 0 {
			온도_상승치 = 0
		}

		시각_문자열 := time.Now().Format("15:04:05")
		온도_예측치 := 현재_온도 + 온도_상승치
		현재_클럭 := f현재_클럭()
		최저_클럭 := 현재_클럭 == f최저_클럭()

		if 기준_온도_초과 {
			과열_발생_클럭 = f최소값(현재_클럭, 과열_발생_클럭)
			f경고음_발생()
			주의_요망 = true
			fmt.Printf("%s : %s. GPU 과열 방지를 위해서 클럭을 최저값으로 낮춥니다. [%vMHz]\n", 시각_문자열, 버퍼.String(), f클럭_변경(f최저_클럭()))
		} else if !최저_클럭 && f기준_온도_근접(기준_온도, 현재_온도, 온도_예측치) {
			주의_요망 = true
			fmt.Printf("%s : %s. 기준 온도 근접. [%vMHz]\n", 시각_문자열, 버퍼.String(), f클럭_한단계_낮추기(현재_클럭))
		} else if !최저_클럭 && f온도_상승_중(기준_온도, 현재_온도, 온도_예측치) {
			주의_요망 = true
			fmt.Printf("%s : %s. 온도 상승 중. [%vMHz]\n", 시각_문자열, 버퍼.String(), f클럭_한단계_낮추기(현재_클럭))
		} else if 최저_클럭 && 현재_온도 < 기준_온도-클럭_회복_기준 {
			클럭_모음 := f지원되는_클럭_모음()
			하한선 := 클럭_모음[len(클럭_모음)-3]

			if 과열_발생_클럭 < 하한선 {
				과열_발생_클럭 = 하한선
			}

			fmt.Printf("%s : %s. 온도가 낮아졌으므로 GPU 클럭을 회복합니다. [%vMHz]\n", 시각_문자열, 버퍼.String(), f클럭_한단계_낮추기(과열_발생_클럭))
		} else {
			fmt.Printf("%s : %s.\n", 시각_문자열, 버퍼.String()) //, strconv.Itoa(int(현재_클럭)))
		}
	}

	return 현재_온도, 주의_요망
}

func f기준_온도_근접(기준_온도, 현재_온도, 온도_예측치 float64) bool {
	기준치1 := 기준_온도 - 3.0
	기준치2 := 기준_온도 - (클럭_회복_기준 / 2.0)
	기준치 := 기준치1

	if 기준치2 < 기준치1 {
		기준치 = 기준치2
	}

	return 현재_온도 > 기준치 && 온도_예측치 > 기준치
}

func f온도_상승_중(기준_온도, 현재_온도, 온도_예측치 float64) bool {
	return 온도_예측치 > 현재_온도 && 온도_예측치 > 기준_온도-클럭_회복_기준
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

// 참고 링크 : https://github.com/golang/go/issues/28804#issuecomment-505326268
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

	token := windows.Token(0)

	if 관리자_여부, 에러 := token.IsMember(sid); 에러 != nil {
		return false
	} else {
		return 관리자_여부
	}
}

func f숫자_추출(문자열 string) string {
	re, _ := regexp.Compile("(\\d+)")
	return re.FindString(문자열)
}

func f최대값[T constraints.Float | constraints.Integer](값_모음 ...T) T {
	if len(값_모음) == 0 {
		panic("입력값이 없습니다.")
	}

	최대값 := 값_모음[0]

	for _, 값 := range 값_모음 {
		if 값 > 최대값 {
			최대값 = 값
		}
	}

	return 최대값
}

func f최소값[T constraints.Float | constraints.Integer](값_모음 ...T) T {
	if len(값_모음) == 0 {
		panic("입력값이 없습니다.")
	}

	최소값 := 값_모음[0]

	for _, 값 := range 값_모음 {
		if 값 < 최소값 {
			최소값 = 값
		}
	}

	return 최소값
}

func f평균값[T constraints.Float | constraints.Integer](값_모음 ...T) T {
	if len(값_모음) == 0 {
		panic("입력값이 없습니다.")
	}

	합계 := T(0)

	for _, 값 := range 값_모음 {
		합계 += 값
	}

	return 합계 / T(len(값_모음))
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

func f클럭_정보_문자열_모음() []string {
	커맨드_클럭 := exec.Command("nvidia-smi", "-q", "-d", "CLOCK")
	출력_바이트_모음, 에러 := 커맨드_클럭.CombinedOutput()
	if 에러 != nil || len(출력_바이트_모음) == 0 {
		fmt.Println(에러)
		return make([]string, 0)
	}

	return strings.Split(string(출력_바이트_모음), "\n")
}

func f현재_클럭() float64 {
	행_모음 := f클럭_정보_문자열_모음()
	인덱스_Application_Clock := -1

	for i, 행 := range 행_모음 {
		if strings.Contains(행, "Applications Clocks") &&
			!strings.Contains(행, "Default Applications Clocks") {
			인덱스_Application_Clock = i
			break
		}
	}

	if 인덱스_Application_Clock < 0 {
		fmt.Println("Application_Clock 미발견")
		return -1.0
	}

	GPU클럭_행 := 행_모음[인덱스_Application_Clock+1]
	if !strings.Contains(GPU클럭_행, "Graphics") {
		fmt.Println("Application Clock GPU값 미발견")
		return -1.0
	}

	if GPU클럭, 에러 := strconv.ParseFloat(f숫자_추출(GPU클럭_행), 64); 에러 != nil {
		return -1.0
	} else {
		return GPU클럭
	}
}

func f클럭_한단계_낮추기(기준_클럭 float64) string {
	if 기준_클럭 <= f최저_클럭() {
		return strconv.Itoa(int(기준_클럭))
	}

	for _, 클럭 := range f지원되는_클럭_모음() {
		if 클럭 < 기준_클럭 {
			return f클럭_변경(클럭)
		}
	}

	return strconv.Itoa(int(기준_클럭))
}

func f클럭_변경(GPU클럭 float64) string {
	if !f관리자_여부() {
		fmt.Printf("** GPU 동작 클럭을 변경하려면 '관리자 권한'이 필요합니다. **\n")
		f관리자_권한으로_재실행()
		close(Ch종료)
	}

	GPU_클럭_문자열 := strconv.Itoa(int(GPU클럭))
	메모리_클럭_문자열 := strconv.Itoa(int(f메모리_클럭()))
	ac인수 := 메모리_클럭_문자열 + "," + GPU_클럭_문자열

	커맨드_실행 := exec.Command("nvidia-smi", "-ac", ac인수)
	커맨드_실행.Run()

	return GPU_클럭_문자열
}

func f메모리_클럭() float64 {
	if 메모리_클럭 == 0.0 {
		f클럭_정보_초기화()
	}

	return 메모리_클럭
}

func f최고_클럭() float64 {
	if len(지원되는_클럭_모음) == 0 {
		f클럭_정보_초기화()
	}

	return 지원되는_클럭_모음[0]
}

func f최저_클럭() float64 {
	if len(지원되는_클럭_모음) == 0 {
		f클럭_정보_초기화()
	}

	return 지원되는_클럭_모음[len(지원되는_클럭_모음)-1]
}

func f지원되는_클럭_모음() []float64 {
	if len(지원되는_클럭_모음) == 0 {
		f클럭_정보_초기화()
	}

	return slices.Clone(지원되는_클럭_모음)
}

func f클럭_정보_초기화() {
	커맨드 := exec.Command("nvidia-smi", "-q", "-d", "SUPPORTED_CLOCKS")
	출력_바이트_모음, 에러 := 커맨드.CombinedOutput()
	if 에러 != nil {
		fmt.Println(에러)
		return
	}

	행_모음 := strings.Split(string(출력_바이트_모음), "\n")

	// 메모리 클럭
	for _, 행 := range 행_모음 {
		if !strings.Contains(행, "Memory") {
			continue
		} else if 숫자값, 에러 := strconv.ParseFloat(f숫자_추출(행), 64); 에러 == nil {
			메모리_클럭 = 숫자값
			break
		}
	}

	// GPU 클럭
	지원되는_클럭_맵 := make(map[float64]bool)

	for _, 행 := range 행_모음 {
		if !strings.Contains(행, "Graphics") {
			continue
		} else if 숫자값, 에러 := strconv.ParseFloat(f숫자_추출(행), 64); 에러 == nil {
			지원되는_클럭_맵[숫자값] = true
		}
	}

	지원되는_클럭_모음_역순 := make([]float64, len(지원되는_클럭_맵), len(지원되는_클럭_맵))

	i := 0
	for 값 := range 지원되는_클럭_맵 {
		지원되는_클럭_모음_역순[i] = 값
		i++
	}

	sort.Float64s(지원되는_클럭_모음_역순)

	지원되는_클럭_모음 = make([]float64, len(지원되는_클럭_모음_역순), len(지원되는_클럭_모음_역순))

	for i := 0; i < len(지원되는_클럭_모음_역순); i++ {
		지원되는_클럭_모음[i] = 지원되는_클럭_모음_역순[len(지원되는_클럭_모음_역순)-1-i]
	}
}
