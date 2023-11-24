FROM golang:1.10 as builder

WORKDIR /workspace
COPY . . 

RUN make

FROM busybox:latest
WORKDIR /
COPY --from=builder /workspace/bin/chatmux /


