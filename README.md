# nvidia-gpu-mon
MS-윈도우 전용 Nvidia GPU 온도 모니터링

기준 온도를 넘어가면 경고음을 내고, 관리자 권한을 획득하여, 동작 클럭을 최저로 낮추어서 과열을 방지한다.
온도가 낮아지면, 동작 속도를 회복한다.
온도 관리 알고리즘이 매우 초보적이지만, 과열 방지 기능은 적절하게 수행하는 것으로 보인다.

Go언어 다운로드 : https://go.dev/dl/

빌드 방법
> go build gpu-mon.go

- 기준 온도를 소스 코드 기본값으로 설정하고 실행
> 윈도우 : gpu-mon.exe 
> 리눅스 : gpu-mon

- 기준 온도를 55도로 설정하고 실행
> 윈도우 : gpu-mon.exe 55
> 리눅스 : gpu-mon 55

