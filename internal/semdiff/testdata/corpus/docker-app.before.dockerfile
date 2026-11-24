FROM golang:1.27 AS build
RUN go build -o /out/app .
ENV LEVEL="debug"
CMD ["/out/app"]
