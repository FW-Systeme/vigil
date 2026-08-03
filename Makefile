.PHONY: build e2e-build e2e clean-e2e test

build:
	go build -o vigil ./cmd/vigil/

e2e-build:
	docker build --network host --platform linux/amd64 -t vigil-e2e -f e2e/Dockerfile .

e2e: e2e-build clean-e2e
	@mkdir -p e2e/logs
	@echo ">>> Running e2e tests (bridge network, auto-start)..."
	docker run --rm --name vigil-e2e-run \
		--privileged --cgroupns=host \
		-v /sys/fs/cgroup:/sys/fs/cgroup:rw \
		-v $(PWD)/e2e/logs:/logs \
		vigil-e2e || true
	@if [ ! -f e2e/logs/result ]; then echo ">>> e2e: no result file written"; exit 1; fi
	@echo ">>> Verdict: $$(cat e2e/logs/result)"
	@[ "$$(cat e2e/logs/result)" = "PASS" ] || { echo ">>> e2e FAILED — details in e2e/logs/"; exit 1; }

clean-e2e:
	-docker stop vigil-e2e-run 2>/dev/null
	-docker rm vigil-e2e-run 2>/dev/null

test:
	go test -race -count=1 ./...
