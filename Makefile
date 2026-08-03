.PHONY: build e2e-bash-build e2e-bash clean-e2e test

build:
	go build -o vigil ./cmd/vigil/

e2e-bash-build:
	docker build --network host --platform linux/amd64 -t vigil-e2e-bash -f e2e/Dockerfile .

e2e-bash: build e2e-bash-build clean-e2e
	@echo "Starting test container..."
	docker run -d --name vigil-e2e-bash-run --privileged --cgroupns=host --network host \
		-v /sys/fs/cgroup:/sys/fs/cgroup:rw \
		-v $(PWD):/src \
		-v $(PWD)/vigil:/usr/local/bin/vigil \
		vigil-e2e-bash /lib/systemd/systemd
	sleep 3
	@echo "Starting nginx..."
	-docker exec vigil-e2e-bash-run nginx 2>/dev/null || true
	@echo "Setting up fixtures..."
	docker exec vigil-e2e-bash-run bash -c "mkdir -p /tmp/vigil-e2e-home && ln -sfn . /src/e2e/fixtures/example-project/app/current"
	@echo "Running e2e tests..."
	docker exec vigil-e2e-bash-run bash /src/e2e/test-example.sh; \
		rc=$$?; \
		docker stop vigil-e2e-bash-run >/dev/null 2>&1; \
		docker rm vigil-e2e-bash-run >/dev/null 2>&1; \
		exit $$rc

clean-e2e:
	-docker stop vigil-e2e-bash-run 2>/dev/null
	-docker rm vigil-e2e-bash-run 2>/dev/null

test:
	go test -race -count=1 ./...
