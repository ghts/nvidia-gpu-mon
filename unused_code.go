package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"slices"
	"sort"
	"strconv"
	"strings"
)

var 지원되는_클럭_모음 []int

func f지원되는_클럭_모음() []int {
	if len(지원되는_클럭_모음) == 0 {
		f지원되는_클럭_모음_초기화()
	}

	return slices.Clone(지원되는_클럭_모음)
}

func f지원되는_클럭_모음_초기화() {
	커맨드 := exec.Command("nvidia-smi", "-q", "-d", "SUPPORTED_CLOCKS")
	출력_바이트_모음, 에러 := 커맨드.CombinedOutput()
	if 에러 != nil {
		fmt.Println(에러)
		return
	}

	지원되는_클럭_맵 := make(map[int]bool, 0)
	전체_바이트_모음 := bytes.Split(출력_바이트_모음, []byte("\n"))

	for _, 행_바이트_모음 := range 전체_바이트_모음 {
		행 := string(행_바이트_모음)

		if !strings.Contains(행, "Graphics") {
			continue
		}

		숫자_문자열 := f숫자_추출(행)

		if 정수, 에러 := strconv.Atoi(숫자_문자열); 에러 == nil {
			지원되는_클럭_맵[정수] = true
		}
	}

	지원되는_클럭_모음 = make([]int, len(지원되는_클럭_맵), len(지원되는_클럭_맵))

	i := 0
	for 값 := range 지원되는_클럭_맵 {
		지원되는_클럭_모음[i] = 값
		i++
	}

	sort.Ints(지원되는_클럭_모음)
	지원되는_클럭_모음 = slices.Clip(지원되는_클럭_모음)
}
