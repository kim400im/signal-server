# --- 1단계: 빌드 환경 ---
# Go 언어 공식 이미지를 'builder'라는 이름의 빌드 환경으로 사용합니다.
FROM golang:1.24-alpine AS builder

# 작업 디렉토리를 /app으로 설정합니다.
WORKDIR /app

# Go 모듈 파일을 먼저 복사하여 Docker의 캐시 기능을 활용합니다.
COPY go.mod ./
COPY go.sum ./

# 의존성 라이브러리를 다운로드합니다.
RUN go mod download

# 나머지 소스 코드를 복사합니다.
COPY . .

# Go 프로그램을 빌드합니다. CGO_ENABLED=0은 외부 라이브러리 의존성 없는 정적 바이너리를 만듭니다.
# -o /server는 결과물을 /server 파일로 저장하라는 의미입니다.
RUN CGO_ENABLED=0 GOOS=linux go build -o /server .

# --- 2단계: 최종 실행 환경 ---
# 아주 가벼운 Alpine Linux를 최종 실행 환경으로 사용합니다.
FROM alpine:latest  

# 작업 디렉토리를 설정합니다.
WORKDIR /root/

# 1단계(builder)에서 빌드한 결과물(/server)을 현재 디렉토리로 복사합니다.
COPY --from=builder /server .

# 서버가 8080 포트를 사용한다고 Docker에 알려줍니다.
EXPOSE 8080

# 컨테이너가 시작될 때 /server 파일을 실행하라는 명령어입니다.
CMD ["./server"]