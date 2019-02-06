ENV =.env
-include $(ENV)
export $(shell sed 's/=.*//' $(ENV))

#LOCAL ENV VARIABLES
VERSION := $(shell cat VERSION)

default: list

list:
	sh -c "echo; $(MAKE) -p no_targets__ | awk -F':' '/^[a-zA-Z0-9][^\$$#\/\\t=]*:([^=]|$$)/ {split(\$$1,A,/ /);for(i in A)print A[i]}' | grep -v '__\$$' | grep -v 'Makefile'| sort"

install-dep:
	$(shell go get -d -u github.com/golang/dep)
	$(eval DEP_LATEST = $(shell cd $(GOPATH)/src/github.com/golang/dep; git describe --abbrev=0 --tags))
	$(shell cd $(GOPATH)/src/github.com/golang/dep; git checkout $(DEP_LATEST))
	$(shell cd $(GOPATH)/src/github.com/golang/dep; go install -ldflags="-X main.version=$(DEP_LATEST)" ./cmd/dep)
	$(shell cp $(GOPATH)/bin/dep $(GOPATH)/src/$(NAMEREPO)/)

build-app:
	cd $(GOPATH)/src/$(NAMEREPO) && $(GOPATH)/bin/dep ensure
	cd $(GOPATH)/src/$(NAMEREPO) && go build -o ./bin/middleware

build-docker:
	@echo '> Build image middleware:$(VERSION)...'
	$(shell ./version.sh)
	$(eval VERSION := $(shell cat VERSION))
	docker build -t middleware:$(VERSION) .

start:
	@echo '> Start middleware...'
	$(shell $(GOPATH)/src/$(NAMEREPO)/bin/middleware > $(GOPATH)/src/$(NAMEREPO)/stdout.txt 2> $(GOPATH)/src/$(NAMEREPO)/stderr.txt &  echo $$! > $(GOPATH)/src/$(NAMEREPO)/middleware.pid)

stop:
	@echo '> Stop middleware...'
	$(shell kill `cat $(GOPATH)/src/$(NAMEREPO)/middleware.pid` && rm -r $(GOPATH)/src/$(NAMEREPO)/middleware.pid)

restart: stop start

