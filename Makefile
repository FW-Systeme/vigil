.PHONY: build e2e-build e2e-run e2e clean-e2e test

build:
	go build -o vigil ./cmd/vigil/

e2e-build: build
	docker build --network host --platform linux/amd64 -t vigil-e2e -f e2e/Dockerfile .

e2e-run:
	@VIGIL_HOME=/tmp/vigil-e2e-home \
	VIRGIL_SYSTEMD_DIR=/tmp/vigil-e2e-systemd \
	VIRGIL_LOGROTATE_DIR=/tmp/vigil-e2e-logrotate \
	VIRGIL_NGINX_AVAILABLE_DIR=/tmp/vigil-e2e-nginx-available \
	VIRGIL_NGINX_ENABLED_DIR=/tmp/vigil-e2e-nginx-enabled \
	go test -tags=e2e -v -count=1 -timeout 15m ./e2e/

e2e: build e2e-run

clean-e2e:
	-docker stop vigil-e2e-run 2>/dev/null

test:
	go test -race -count=1 ./...
