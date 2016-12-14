.PHONY: install build build-all

NAME:=nmp
PACKAGE:=milk/nmp

install:
	go install github.com/$(PACKAGE)/cmd/$(NAME)

build:
	go build github.com/$(PACKAGE)/cmd/$(NAME)

build-all:
	env GOOS=darwin GOARCH=amd64 go build -o dist/$(NAME)_$(VERSION)_darwin_amd64/$(NAME) github.com/$(PACKAGE)/cmd/$(NAME) && \
		cd dist && \
		tar cvzf $(NAME)_$(VERSION)_darwin_amd64.tgz $(NAME)_$(VERSION)_darwin_amd64 && \
		rm -rf $(NAME)_$(VERSION)_darwin_amd64
	env GOOS=linux GOARCH=amd64 go build -o dist/$(NAME)_$(VERSION)_linux_amd64/$(NAME) github.com/$(PACKAGE)/cmd/$(NAME) && \
		cd dist && \
		tar cvzf $(NAME)_$(VERSION)_linux_amd64.tgz $(NAME)_$(VERSION)_linux_amd64 && \
		rm -rf $(NAME)_$(VERSION)_linux_amd64
	env GOOS=linux GOARCH=386 go build -o dist/$(NAME)_$(VERSION)_linux_386/$(NAME) github.com/$(PACKAGE)/cmd/$(NAME) && \
		cd dist && \
		tar cvzf $(NAME)_$(VERSION)_linux_386.tgz $(NAME)_$(VERSION)_linux_386 && \
		rm -rf $(NAME)_$(VERSION)_linux_386
