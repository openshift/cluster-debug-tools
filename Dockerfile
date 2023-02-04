FROM golang:1.19-alpine
RUN apk add alpine-sdk
RUN mkdir -p /app/cluster-debug-tools
WORKDIR /app/cluster-debug-tools
CMD git clone https://github.com/openshift/cluster-debug-tools.git . && make

