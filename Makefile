ENV =.env
-include $(ENV)
export $(shell sed 's/=.*//' $(ENV))

build:
	cd $(GOPATH)/src/$(NAMEREPO) && $(GOPATH)/bin/dep ensure
	cd $(GOPATH)/src/$(NAMEREPO) && go build -o ./bin/middleware

start:
	$(GOPATH)/src/$(NAMEREPO)/bin/middleware > $(GOPATH)/src/$(NAMEREPO)/stdout.txt 2> $(GOPATH)/src/$(NAMEREPO)/stderr.txt &  echo $$! > $(GOPATH)/src/$(NAMEREPO)/middleware.pid

stop:
	kill `cat $(GOPATH)/src/$(NAMEREPO)/middleware.pid` && rm -r $(GOPATH)/src/$(NAMEREPO)/middleware.pid

restart: stop start
