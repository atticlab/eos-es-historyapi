ENV =.env
COMPOSE = docker-compose.yml
-include $(ENV)
export $(shell sed 's/=.*//' $(ENV))

#LOCAL ENV VARIABLES
VERSION := $(shell cat VERSION)

define composer-generator
   sed -i 's|{{MIDDLEWARE_SOURCE_PORT}}|$(MIDDLEWARE_SOURCE_PORT)|g' $(COMPOSE)
   sed -i 's|{{MIDDLEWARE_DEST_PORT}}|$(MIDDLEWARE_DEST_PORT)|g' $(COMPOSE)
   $(eval VERSION := $(shell cat VERSION))
   sed -i 's|middleware\:.*|middleware:$(VERSION)|g' $(COMPOSE)
endef

default: list

list:
	@sh -c "echo; $(MAKE) -p no_targets__ | awk -F':' '/^[a-zA-Z0-9][^\$$#\/\\t=]*:([^=]|$$)/ {split(\$$1,A,/ /);for(i in A)print A[i]}' | grep -v '__\$$' | grep -v 'Makefile'| sort"

generate:
	$(composer-generator)

install-dep:
	@echo '> Installing dep...'
	$(shell go get -d -u github.com/golang/dep)
	$(eval DEP_LATEST = $(shell cd $(GOPATH)/src/github.com/golang/dep; git describe --abbrev=0 --tags))
	$(shell cd $(GOPATH)/src/github.com/golang/dep; git checkout $(DEP_LATEST))
	$(shell cd $(GOPATH)/src/github.com/golang/dep; go install -ldflags="-X main.version=$(DEP_LATEST)" ./cmd/dep)
	$(shell cp $(GOPATH)/bin/dep $(GOPATH)/src/$(NAMEREPO)/)

build-app: install-dep
	@echo '> Build middleware app...'
	dep ensure
	cd $(GOPATH)/src/$(NAMEREPO) && go build -o ./bin/middleware

build-docker: install-dep
ifneq (,$(wildcard config.json))
	@echo '> Build image middleware:$(VERSION)...'
	$(shell ./version.sh)
	$(eval VERSION := $(shell cat VERSION))
	docker build -t middleware:$(VERSION) .
else
	@echo 'File config.json is not exist. Please create a config.json file.'
endif

create-compose: build-docker generate

docker-start:
	docker-compose up -d

docker-stop:
	docker-compose stop

start:
ifneq (,$(wildcard config.json))
	@echo '> Start middleware...'
	$(shell $(GOPATH)/src/$(NAMEREPO)/bin/middleware > $(GOPATH)/src/$(NAMEREPO)/stdout.txt 2> $(GOPATH)/src/$(NAMEREPO)/stderr.txt &  echo $$! > $(GOPATH)/src/$(NAMEREPO)/middleware.pid)
else
	@echo 'File config.json is not exist. Please create a config.json file.'
endif

stop:
	@echo '> Stop middleware...'
	$(shell kill `cat $(GOPATH)/src/$(NAMEREPO)/middleware.pid` && rm -r $(GOPATH)/src/$(NAMEREPO)/middleware.pid)

restart: stop start

