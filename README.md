# DEPRECATED
'nvidia-pstated'를 사용하세요. 대기 전력이 낮아집니다.
https://github.com/sasha0552/nvidia-pstated/

# nvidia-gpu-mon
Nvidia GPU 온도 모니터링<br>
MS-윈도우 및 리눅스 지원됨.<br>
GPU온도 변화에 따라 과열되지 않도록 동작 클럭을 조절한다.<br>
온도 관리 알고리즘이 초보적이지만, 과열 방지라는 목표는 적절하게 달성하는 것으로 보인다.<br>

Go언어 다운로드 : https://go.dev/dl/

빌드 방법
> go build gpu-mon.go

- 기준 온도를 소스 코드 기본값으로 설정하고 실행
> 윈도우 : gpu-mon.exe 
> 리눅스 : gpu-mon

- 기준 온도를 55도로 설정하고 실행
> 윈도우 : gpu-mon.exe 55
> 리눅스 : gpu-mon 55

