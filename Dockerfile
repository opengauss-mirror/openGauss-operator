#Copyright (c) 2021 opensource@cmbc.com.cn
#OpenGauss Operator is licensed under Mulan PSL v2.
#You can use this software according to the terms and conditions of the Mulan PSL v2.
#You may obtain a copy of Mulan PSL v2 at:
#         http://license.coscl.org.cn/MulanPSL2
#THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND,
#EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT,
#MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
#See the Mulan PSL v2 for more details.

# Build the manager binary
FROM golang:1.15 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
arg GO111MODULE=on
arg GOPROXY='https://goproxy.cn'
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY utils/ utils/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o opengauss-operator main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
#operator生成的默认Dockerfile会指定使用distroless做镜像基础文件，但我们无法访问原有的地址，因此需要修改为
#FROM kubeimages/distroless-static
FROM exploitht/operator-static
WORKDIR /
COPY --from=builder /workspace/opengauss-operator /usr/local/bin/opengauss-operator
USER 65532:65532

ENTRYPOINT ["/usr/local/bin/opengauss-operator"]


