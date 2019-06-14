.PHONY: build
VERSION=0.0.4

build: COMMIT=$(shell git rev-list -1 HEAD | grep -o "^.\{10\}")
build: DATE=$(shell date +'%Y-%m-%d %H:%M')
build: 
	go get
	go get github.com/abice/go-enum
	go generate ./...
	env GOOS=darwin  GOARCH=amd64 go build -ldflags '-X "main.Version=$(VERSION) ($(COMMIT) - $(DATE))"' -o build/$(VERSION)/ec2-$(VERSION)-darwin
	env GOOS=linux   GOARCH=amd64 go build -ldflags '-X "main.Version=$(VERSION) ($(COMMIT) - $(DATE))"' -o build/$(VERSION)/ec2-$(VERSION)-linux
	env GOOS=windows GOARCH=amd64 go build -ldflags '-X "main.Version=$(VERSION) ($(COMMIT) - $(DATE))"' -o build/$(VERSION)/ec2-$(VERSION)-windows.exe

test:
	go run main.go run \
		--ami-filter "owner-alias=amazon" \
		--ami-filter "name=amzn2-ami-hvm*x86_64-ebs" \
		--tag "Name=Hello World" \
		--subnet-filter "tag:Environment=qa" \
		--subnet-filter "tag:Type=private" \
		--security-group-filter "group-name=qa_private" \
		--type t2.micro \
		echo "Hello world"

publish:
	rsync -a build/ /keybase/public/justmiles/artifacts/ec2-cli/