FROM golang:1.16-alpine AS build
ENV GOOS=linux
ENV CGO_ENABLED=0
WORKDIR /src
COPY go.* .
RUN go mod download -x
COPY *.go /src/
RUN go build -o /go/bin/image-clone-controller

FROM gcr.io/distroless/base-debian10
WORKDIR /
COPY --from=build /go/bin/image-clone-controller /image-clone-controller
USER nonroot:nonroot
ENTRYPOINT ["/image-clone-controller"]
