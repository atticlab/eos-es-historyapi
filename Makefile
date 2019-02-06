ENV =.env
-include $(ENV)
export $(shell sed 's/=.*//' $(ENV))

#LOCAL ENV VARIABLES
VERSION := $(shell cat VERSION)

default: list

list:
	sh -c "echo; $(MAKE) -p no_targets__ | awk -F':' '/^[a-zA-Z0-9][^\$$#\/\\t=]*:([^=]|$$)/ {split(\$$1,A,/ /);for(i in A)print A[i]}' | grep -v '__\$$' | grep -v 'Makefile'| sort"

build-app:
	cd $(GOPATH)/src/$(NAMEREPO) && $(GOPATH)/bin/dep ensure
	cd $(GOPATH)/src/$(NAMEREPO) && go build -o ./bin/middleware

start:
	$(GOPATH)/src/$(NAMEREPO)/bin/middleware > $(GOPATH)/src/$(NAMEREPO)/stdout.txt 2> $(GOPATH)/src/$(NAMEREPO)/stderr.txt &  echo $$! > $(GOPATH)/src/$(NAMEREPO)/middleware.pid

stop:
	kill `cat $(GOPATH)/src/$(NAMEREPO)/middleware.pid` && rm -r $(GOPATH)/src/$(NAMEREPO)/middleware.pid

restart: stop start

