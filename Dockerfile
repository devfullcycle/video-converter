FROM golang:1.22.0-alpine3.19 as builder
WORKDIR /build 
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -o app 
 
FROM ubuntu:22.04 as final
RUN  apt update && apt install ffmpeg --yes
WORKDIR /app
COPY --from=builder /build/app .
CMD [ "./app" ]
