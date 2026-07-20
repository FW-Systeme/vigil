.PHONY: build e2e-build e2e-run e2e clean-e2e test

build:
	go build -o vigil ./cmd/vigil/

e2e-build:
	docker build --network host --platform linux/amd64 -t vigil-e2e -f e2e/Dockerfile .

e2e-run: build
	@echo "Starting test container..."
	docker run -d --name vigil-e2e-run --privileged --cgroupns=host --network host \
		-v /sys/fs/cgroup:/sys/fs/cgroup:rw \
		-v $(PWD):/src \
		-v $(PWD)/vigil:/usr/local/bin/vigil \
		-v $(shell go env GOMODCACHE):/go/pkg/mod \
		vigil-e2e /lib/systemd/systemd
	sleep 3
	@echo "Starting nginx..."
	-docker exec vigil-e2e-run nginx 2>/dev/null || true
	@echo "Creating temp dirs and fixture setup..."
	docker exec vigil-e2e-run bash -c "mkdir -p /tmp/vigil-e2e-home /tmp/vigil-e2e-logrotate && ln -sfn . /src/e2e/fixtures/dummy-app/current && mkdir -p /src/e2e/fixtures/dummy-app/shared"
	@echo "Running e2e tests inside container..."
	-docker exec \
		-e VIRGIL_E2E_FIXTURES=/src/e2e/fixtures \
		-e VIRGIL_HOME=/tmp/vigil-e2e-home \
		vigil-e2e-run \
		go test -tags=e2e -v -count=1 -timeout 15m ./e2e/
	@echo "Cleaning up..."
	-docker stop vigil-e2e-run 2>/dev/null
	-docker rm vigil-e2e-run 2>/dev/null

e2e: e2e-run

clean-e2e:
	-docker stop vigil-e2e-run 2>/dev/null
	-docker rm vigil-e2e-run 2>/dev/null

test:
	go test -race -count=1 ./...
