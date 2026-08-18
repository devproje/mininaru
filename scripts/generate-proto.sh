#!/bin/sh

set -eu

PROTOC=${PROTOC:-protoc}

PATH="$(go env GOPATH)/bin:$PATH"
export PATH

test "$("$PROTOC" --version)" = "libprotoc 35.0"
test "$(protoc-gen-go --version)" = "protoc-gen-go v1.36.11"
test "$(protoc-gen-go-grpc --version)" = "protoc-gen-go-grpc 1.6.2"

"$PROTOC" -I api \
    --go_out=. --go_opt=module=github.com/devproje/mininaru \
    --go-grpc_out=. --go-grpc_opt=module=github.com/devproje/mininaru \
    api/mininaru/v1/mininaru.proto

for file in rpc/gen/mininaru/v1/mininaru.pb.go rpc/gen/mininaru/v1/mininaru_grpc.pb.go; do
    temp="${file}.tmp"
    {
        echo '// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)'
        echo '// SPDX-License-Identifier: GPL-3.0-or-later'
        awk '!/^[[:space:]]*\/\//' "$file"
    } > "$temp"
    mv "$temp" "$file"
done

gofmt -w rpc/gen/mininaru/v1/mininaru.pb.go rpc/gen/mininaru/v1/mininaru_grpc.pb.go
go run ./scripts/protostyle
