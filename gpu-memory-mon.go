//go:build memory

package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	lb "github.com/ghts/ghts/lib"
	gm "github.com/ghts/nvidia-gpu-mon/gpu_mon"
)

// LLM 디코딩 단계에서 연산량은 낮고, 메모리 입출력 데이터량은 많은 데,
// GPU 연산량이 낮아지면 자동으로 GPU 클럭이 낮아지는 데, 이 때 덩달아 메모리 클럭도 낮아짐.
// LLM 디코딩 단계의 병목 지점인 메모리 동작 속도가 낮아지면, 병목이 더 심해지는 악순환이 발생함.
// 이러한 악순환을 방지하기 위하여 GPU 메모리 로드가 높아지면 메모리 클럭을 높게 유지함.
// 모든 화면 출력은 iGPU에서 처리하고, Nvidia GPU에 LLM 엔진 이외의 일체 다른 프로세스가 처리되지 않도록 설정해야 함.
// MS윈도우 '설정 - 시스템 - 디스플레이 - 그래픽' 에서 GPU 선택 가능함.

const (
	목표_클럭_하한     = "9752" // 고정할 메모리 클럭 값 (원하는 숫자로 수정)
	목표_클럭_상한     = "10502"
	체크_주기        = lb.P10초 // 체크 주기
	메모리_클럭_해제_기준 = 3       // 이 횟수만큼 연속으로 'GPU메모리_로드 <= 1'이면 클럭 해제
)

const (
	P상태_미정 T상태 = iota
	P상태_유휴
	P상태_로드
)

type T상태 uint8

func main() {
	if !gm.F관리자_여부() {
		fmt.Printf("** GPU 설정을 변경하려면 '관리자 권한'이 필요합니다. **\n")
		gm.F관리자_권한으로_재실행()
		return
	}

	fmt.Println("GPU 메모리 클럭 자동 조절 프로그램을 시작합니다. (종료: Ctrl + C)")
	마지막_상태 := P상태_미정
	유휴_상태_연속_횟수 := 0 // 유휴 상태가 연속으로 관측된 횟수

	for {
		지금 := lb.F지금().Format(lb.P시분초_형식)

		// GPU 메모리 로드에 따른 분기 및 중복 실행 방지 처리
		if GPU메모리_로드, 에러 := gpu메모리_로드(); 에러 != nil {
			fmt.Printf("[%s] 오류 : GPU 메모리 로드 획득 실패\n\n%v\n\n", 지금, 에러)
			유휴_상태_연속_횟수 = 0 // 측정 실패 시 연속 카운터 초기화
		} else if GPU메모리_로드 <= 1 {
			if 유휴_상태_연속_횟수 < 메모리_클럭_해제_기준 {
				유휴_상태_연속_횟수++
			}

			if 유휴_상태_연속_횟수 >= 메모리_클럭_해제_기준 && 마지막_상태 == P상태_로드 {
				if 에러 = gpu메모리_클럭_초기화(); 에러 != nil {
					fmt.Printf("[%s] 오류 : 메모리 클럭 리셋 실패\n\n%v\n\n", 지금, 에러)
				} else {
					fmt.Printf("[%s] GPU 메모리 클럭 잠금 해제\n", 지금)
					마지막_상태 = P상태_유휴

					// LLM 작업이 종료되었으므로 사용자에게 알림음 발생.
					go func() {
						if lb.F지금().Hour() < 8 {
							return // 새벽 시간에는 알림 소리 내리 않음.
						}

						for range 10 {
							gm.F경고음_발생()
							lb.F대기(lb.P10초)
						}
					}()
				}
			}
		} else {
			유휴_상태_연속_횟수 = 0 // 메모리 로드가 높아지면 연속 카운터 초기화

			if 마지막_상태 != P상태_로드 {
				if 에러 = gpu메모리_클럭_고정(); 에러 != nil {
					fmt.Printf("[%s] 오류 : 메모리 클럭 잠금 실패\n\n%v\n\n", 지금, 에러)
				} else {
					fmt.Printf("[%s] GPU 메모리 클럭 잠금 활성화\n", 지금)
					마지막_상태 = P상태_로드
				}
			}
		}

		lb.F대기(체크_주기)
	}
}

// gpu메모리_로드 : nvidia-smi를 사용하여 GPU 메모리 로드 가져오기
// GPU가 여러 개이면 각 GPU의 메모리 로드의 최대값을 반환한다.
// utilization.memory는 '샘플링 기간 동안 GPU 메모리 버스(컨트롤러)가 활발하게 작동한 시간의 비율'
// memory.used는 '사용 중인 메모리 양'
func gpu메모리_로드() (int, error) {
	var out, errOut bytes.Buffer

	커맨드 := exec.Command("nvidia-smi.exe", "--query-gpu=utilization.memory", "--format=csv,noheader,nounits")
	커맨드.Stdout = &out
	커맨드.Stderr = &errOut

	에러 := 커맨드.Run()
	if 에러 != nil {
		return 0, lb.New에러("[오류] nvidia-smi 실행 실패 (Path 확인 필요)\n%v\n(stderr)\n%s",
			에러, lb.F2문자열_공백_제거(errOut.String()))
	}

	// 여러 GPU이면 줄단위로 나열되므로, 각 줄을 개별 로드로 변환해 최대값을 구한다.
	메모리_로드_모음 := make([]int, 0)
	for _, 줄 := range strings.Split(out.String(), "\n") {
		if 정제된_값 := lb.F2문자열_공백_제거(줄); 정제된_값 == "" {
			continue
		} else if 메모리_로드, 변환_에러 := strconv.Atoi(정제된_값); 변환_에러 != nil {
			return 0, lb.New에러("[오류] 메모리 로드 변환 실패 (%q) : %v", 정제된_값, 변환_에러)
		} else {
			메모리_로드_모음 = append(메모리_로드_모음, 메모리_로드)
		}
	}

	if len(메모리_로드_모음) == 0 {
		return 0, lb.New에러("[오류] nvidia-smi로부터 GPU 메모리 로드를 획득하지 못했습니다.")
	}

	return lb.F최대값(메모리_로드_모음...), nil
}

func gpu메모리_클럭_초기화() error {
	커맨드 := exec.Command("nvidia-smi.exe", "--reset-memory-clocks")

	if 출력, 에러 := 커맨드.CombinedOutput(); 에러 != nil {
		return lb.New에러("[오류] 메모리 클럭 리셋 실패 : %v\n(stderr)\n%s", 에러, lb.F2문자열_공백_제거(string(출력)))
	}

	return nil
}

func gpu메모리_클럭_고정() error {
	잠금_인수 := fmt.Sprintf("%s,%s", 목표_클럭_하한, 목표_클럭_상한)
	커맨드 := exec.Command("nvidia-smi.exe", "--lock-memory-clocks="+잠금_인수)

	if 출력, 에러 := 커맨드.CombinedOutput(); 에러 != nil {
		return lb.New에러("[오류] 메모리 클럭 잠금 실패 : %v\n(stderr)\n%s", 에러, lb.F2문자열_공백_제거(string(출력)))
	}
	return nil
}
