package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var 기준_온도 float64 = 48 // 섭씨

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
		} else {
			fmt.Printf("%s : %s\n", 시각_문자열, 버퍼.String())
		}
	}
}

func main() {
	if len(os.Args) > 1 {
		for _, 인수 := range os.Args[1:] {
			if 값, 에러 := strconv.ParseFloat(인수, 64); 에러 == nil {
				기준_온도 = 값
				break
			}
		}
	}

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
