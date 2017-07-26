.PHONY: build
build: ; bash dev.sh build

.PHONY: cluster-upgrade
cluster-upgrade: ; bash dev.sh cluster-upgrade

.PHONY: cluster-build
cluster-build: ; bash dev.sh cluster-build

.PHONY: cluster-start
cluster-start: ; bash dev.sh cluster-start

.PHONY: cluster-stop
cluster-stop: ; bash dev.sh cluster-stop

.PHONY: cli-build
cli-build: ; bash dev.sh cli-build

.PHONY: frontend-build
frontend-build: ; bash dev.sh frontend-build

.PHONY: frontend-start
frontend-start: ; bash dev.sh frontend-start

.PHONY: frontend-stop
frontend-stop: ; bash dev.sh frontend-stop

.PHONY: frontend-dist
frontend-dist: ; bash dev.sh frontend-dist