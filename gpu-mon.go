package main

import (
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

	반환값, _, _ := proc.Call(0xFFFFFFFF) // 0xFFFFFFFF is the frequency for a standard f경고음_발생 sound
	if 반환값 == 0 {
		fmt.Println("Failed to produce f경고음_발생 sound")
	}
}

func gpu온도_확인() {
	if 온도_모음, 에러 := gpu온도_측정(); 에러 == nil {
		비프음_발생_여부 := false

		for i, 온도 := range 온도_모음 {
			if 온도 > 기준_온도 {
				비프음_발생_여부 = true // 온도가 50를 넘어가면 비프음 낸다.
				fmt.Printf("GPU %d 과열 : %.1f°C (!!)\n", i, 온도)
			} else {
				fmt.Printf("GPU %d 온도 : %.1f°C\n", i, 온도)
			}
		}

		if 비프음_발생_여부 {
			f경고음_발생()
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
