FROM golang:1.20.6-alpine3.18 as builder

ENV SRC_DIR=/go/src/github.com/kyma-project/control-plane/tests/e2e/provisioning

WORKDIR $SRC_DIR
COPY . $SRC_DIR

RUN CGO_ENABLED=0 GOOS=linux go test -ldflags="-s -w" -c ./test

FROM alpine:3.17.3
LABEL source=git@github.com:kyma-project/control-plane.git

WORKDIR /app

RUN apk --no-cache add ca-certificates curl

COPY --from=builder /go/src/github.com/kyma-project/control-plane/tests/e2e/provisioning/test.test .

ENTRYPOINT ["/app/test.test"]
