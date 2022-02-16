all: local

local:
	GOOS=linux GOARCH=amd64 go build -o=kube-scheduler ./cmd/scheduler

build:

	sudo docker build --no-cache . -t centaurusinfra/vgpu-scheduler:1.0.2

push:
	sudo docker push centaurusinfra/vgpu-scheduler:1.0.2

# Run go fmt against code
fmt:
	sudo gofmt -l -w .

clean: fmt vet
	sudo rm -f kube-scheduler