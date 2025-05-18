FROM golang:1.24

WORKDIR /app
COPY . .

RUN go build -o mychat-room .

EXPOSE 5000

CMD ["./mychat-room"]
