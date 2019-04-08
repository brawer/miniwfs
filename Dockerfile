# Copyright (c) 2019 Sascha Brawer
# SPDX-License-Identifier: MIT
#
# $ docker build -t brawer/miniwfs .
#
# $ docker run -p 8080:8080 --mount type=bind,source=/Users/sascha/src/miniwfs/data,target=/var/miniwfs -it brawer/miniwfs --collections castles=/var/miniwfs/castles.geojson
#
# $ curl http://localhost:8080/collections
# $ curl http://localhost:8080/collections/castles/items?bbox=11.18,47.91,11.19,47.92
# $ curl http://localhost:8080/collections/castles/items/W548140156
# $ curl http://localhost:8080/metrics

FROM golang:1.12-alpine3.9 as builder
WORKDIR /src/miniwfs
RUN apk --no-cache add build-base git
COPY . ./
RUN go mod download
RUN CGO_ENABLED=1 go build -a -o miniwfs .
RUN CGO_ENABLED=1 go test

FROM alpine:3.9
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /src/miniwfs/miniwfs .

EXPOSE 8080
VOLUME ["/var/miniwfs"]
ENTRYPOINT ["./miniwfs", "--port=8080"]
CMD ["--collections=castles=/var/miniwfs/castles.geojson"]
