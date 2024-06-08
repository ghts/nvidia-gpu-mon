# nvidia-gpu-mon
MS-윈도우 전용 Nvidia GPU 온도 모니터링

기준 온도를 넘어가면 경고음을 낸다.

Go언어 다운로드 : https://go.dev/dl/

빌드 방법
> go build gpu-mon.go

- 기준 온도를 소스 코드 기본값으로 설정하고 실행
> gpu-mon.exe 

- 기준 온도를 55도로 설정하고 실행
> gpu-mon.exe 55

