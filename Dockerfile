FROM public.ecr.aws/docker/library/golang:1.18.7

WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . ./
RUN go build -v -o validation_bot cmd/validation_bot.go

EXPOSE 80
EXPOSE 7999
EXPOSE 7998
CMD ["/app/validation_bot", "run"]
