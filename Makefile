.PHONY: build
build: ; bash dev.sh build

.PHONY: cluster.build
cluster.build: ; bash dev.sh cluster-build

.PHONY: cluster.prodbuild
cluster.prodbuild: ; bash dev.sh cluster-prodbuild

.PHONY: cluster.start
cluster.start: ; bash dev.sh cluster-start

.PHONY: cluster.stop
cluster.stop: ; bash dev.sh cluster-stop

.PHONY: cluster.upgrade
cluster.upgrade: ; bash dev.sh cluster-upgrade

.PHONY: cli.build
cli.build:
	bash dev.sh cli-build
	@echo
	@echo "dm binary created - copy it to /usr/local/bin with this command:"
	@echo
	@echo 'sudo cp -f ./binaries/$$(uname -s |tr "[:upper:]" "[:lower:]")/dm /usr/local/bin'

.PHONY: frontend.build
frontend.build: ; bash dev.sh frontend-build

.PHONY: frontend.start
frontend.start: ; bash dev.sh frontend-start

.PHONY: frontend.stop
frontend.stop: ; bash dev.sh frontend-stop

.PHONY: frontend.dist
frontend.dist: ; bash dev.sh frontend-dist

.PHONY: frontend.test.build
frontend.test.build: ; bash dev.sh frontend-test-build

.PHONY: frontend.test.prod
frontend.test.prod: ; bash dev.sh frontend-test-prod

.PHONY: frontend.test
frontend.test: ; bash dev.sh frontend-test

.PHONY: frontend.dev
frontend.dev: ; CLI=1 make frontend.start

.PHONY: frontend.link
frontend.link: ; CLI=1 LINKMODULES=1 make frontend.start

.PHONY: frontend.cli
frontend.cli: ; docker exec -ti datamesh-frontend bash

.PHONY: help
help: ; docker exec -ti datamesh-frontend yarn run buildhelp

.PHONY: frontend.logs
frontend.logs: ; docker logs -f datamesh-frontend

.PHONY: chromedriver.start
chromedriver.start: ; bash dev.sh chromedriver-start

.PHONY: chromedriver.start.prod
chromedriver.start.prod: ; bash dev.sh chromedriver-start-prod

.PHONY: chromedriver.stop
chromedriver.stop: ; bash dev.sh chromedriver-stop

.PHONY: gotty.start
gotty.start: ; bash dev.sh gotty-start

.PHONY: gotty.stop
gotty.stop: ; bash dev.sh gotty-stop

.PHONY: prod
prod:
	make frontend.build
	make frontend.dist
	make cluster.build
	make cluster.prodbuild
	make cluster.start

.PHONY: reset
reset: ; bash dev.sh reset
