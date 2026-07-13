HOST_OS:=$(shell go env GOHOSTOS)
HOST_ARCH:=$(shell go env GOHOSTARCH)
GIT_TAG:=$(shell git describe --tags --abbrev=0 2>/dev/null || echo dev)
PREFIX:=pst_${GIT_TAG}
EXT:=
SAV_CLI_PYTHON?=python3
SAV_CLI_WINDOWS_AMD64?=artifacts/sav_cli/windows_amd64/sav_cli.exe
SAV_CLI_LINUX_AMD64?=artifacts/sav_cli/linux_amd64/sav_cli
SAV_CLI_LINUX_ARM64?=artifacts/sav_cli/linux_arm64/sav_cli
SAV_CLI_LICENSE?=artifacts/sav_cli/sav_cli-GPL-3.0.txt
RSRC_VERSION?=v0.10.2
ifeq ($(HOST_OS),windows)
	EXT := .exe
endif

.PHONY: init
# 初始化
init:
	go mod download

.PHONY: sav-cli
# 为当前平台构建 Palworld 1.0 兼容的存档解析器
sav-cli:
	mkdir -p dist/
	@if command -v uv >/dev/null 2>&1; then \
		uv run --python 3.13 \
			--with pyinstaller==6.16.0 \
			--with requests==2.32.5 \
			--with orjson==3.11.8 \
			--with setuptools==80.9.0 \
			--with wheel==0.45.1 \
			python script/build_sav_cli.py --output ./dist/sav_cli${EXT}; \
	else \
		$(SAV_CLI_PYTHON) script/build_sav_cli.py --output ./dist/sav_cli${EXT}; \
	fi

.PHONY: windows-resources
# 生成带应用图标的 Windows amd64 资源文件
windows-resources:
	go run github.com/akavel/rsrc@${RSRC_VERSION} -arch amd64 -ico build/windows/pst.ico -o rsrc_windows_amd64.syso
	go run github.com/akavel/rsrc@${RSRC_VERSION} -arch amd64 -ico build/windows/pst.ico -o cmd/pst-agent/rsrc_windows_amd64.syso

.PHONY: build
# 构建当前平台主程序
build: $(if $(filter windows,$(HOST_OS)),windows-resources)
	rm -rf dist/ && mkdir -p dist/
	python3 map_down.py
	rm -rf assets && rm -rf index.html
	cd web && pnpm i --frozen-lockfile && pnpm build && cd ..
	cp example/config.yaml dist/config.yaml
	cp script/start.bat dist/start.bat
	cp THIRD_PARTY_NOTICES.md dist/THIRD_PARTY_NOTICES.md
	$(MAKE) sav-cli
	go build -tags assets -ldflags="-s -w -X 'main.version=${GIT_TAG}'" -o ./dist/pst${EXT} .

.PHONY: frontend
# 仅构建嵌入式前端
frontend:
	rm -rf assets && rm -rf index.html
	cd web && pnpm i --frozen-lockfile && pnpm build && cd ..

.PHONY: build-pub
# 为所有支持平台构建主程序和代理
build-pub: windows-resources
	rm -rf dist/ && mkdir -p dist/
	python3 map_down.py
	rm -rf assets && rm -rf index.html
	cd web && pnpm i --frozen-lockfile && pnpm build && cd ..

	mkdir -p dist/windows_x86_64 && mkdir -p dist/linux_x86_64 && mkdir -p dist/linux_aarch64
	test -f ${SAV_CLI_WINDOWS_AMD64}
	test -f ${SAV_CLI_LINUX_AMD64}
	test -f ${SAV_CLI_LINUX_ARM64}
	test -f ${SAV_CLI_LICENSE}
	cp ${SAV_CLI_WINDOWS_AMD64} ./dist/windows_x86_64/sav_cli.exe
	cp ${SAV_CLI_LINUX_AMD64} ./dist/linux_x86_64/sav_cli
	cp ${SAV_CLI_LINUX_ARM64} ./dist/linux_aarch64/sav_cli
	cp ${SAV_CLI_LICENSE} ./dist/windows_x86_64/sav_cli-GPL-3.0.txt
	cp ${SAV_CLI_LICENSE} ./dist/linux_x86_64/sav_cli-GPL-3.0.txt
	cp ${SAV_CLI_LICENSE} ./dist/linux_aarch64/sav_cli-GPL-3.0.txt
	GOOS=windows GOARCH=amd64 go build -tags assets -ldflags="-s -w -X 'main.version=${GIT_TAG}'" -o ./dist/windows_x86_64/pst.exe .
	GOOS=linux GOARCH=amd64 go build -tags assets -ldflags="-s -w -X 'main.version=${GIT_TAG}'" -o ./dist/linux_x86_64/pst .
	GOOS=linux GOARCH=arm64 go build -tags assets -ldflags="-s -w -X 'main.version=${GIT_TAG}'" -o ./dist/linux_aarch64/pst .

	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o ./dist/pst-agent_${GIT_TAG}_windows_x86_64.exe ./cmd/pst-agent/main.go
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ./dist/pst-agent_${GIT_TAG}_linux_x86_64 ./cmd/pst-agent/main.go
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o ./dist/pst-agent_${GIT_TAG}_linux_aarch64 ./cmd/pst-agent/main.go

	cp example/config.yaml dist/windows_x86_64/config.yaml
	cp example/config.yaml dist/linux_x86_64/config.yaml
	cp example/config.yaml dist/linux_aarch64/config.yaml
	cp script/start.bat dist/windows_x86_64/start.bat
	cp script/start.sh dist/linux_x86_64/start.sh
	cp script/start.sh dist/linux_aarch64/start.sh
	cp THIRD_PARTY_NOTICES.md dist/windows_x86_64/THIRD_PARTY_NOTICES.md
	cp THIRD_PARTY_NOTICES.md dist/linux_x86_64/THIRD_PARTY_NOTICES.md
	cp THIRD_PARTY_NOTICES.md dist/linux_aarch64/THIRD_PARTY_NOTICES.md
	chmod +x dist/linux_x86_64/start.sh dist/linux_aarch64/start.sh

	cd dist && zip -p -r ${PREFIX}_windows_x86_64.zip windows_x86_64/* && tar -czf ${PREFIX}_linux_x86_64.tar.gz linux_x86_64/* && tar -czf ${PREFIX}_linux_aarch64.tar.gz linux_aarch64/* && cd ..

# show help
help:
	@echo ''
	@echo 'Usage:'
	@echo ' make [target]'
	@echo ''
	@echo 'Targets:'
	@awk '/^[a-zA-Z\-\_0-9]+:/ { \
	helpMessage = match(lastLine, /^# (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")-1); \
			helpMessage = substr(lastLine, RSTART + 2, RLENGTH); \
			printf "\033[36m%-22s\033[0m %s\n", helpCommand,helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help
