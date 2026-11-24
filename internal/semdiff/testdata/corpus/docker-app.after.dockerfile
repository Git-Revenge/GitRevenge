FROM golang:1.27 AS build
RUN go build -trimpath -o /out/app .
ENV LEVEL="debug"
CMD [ "/out/app" ]
