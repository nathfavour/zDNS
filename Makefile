PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin

.PHONY: all build clean install uninstall test

all: build

build:
	mkdir -p build
	go build -o build/zdns ./cmd/zdns

test:
	go test -v ./pkg/...

clean:
	rm -rf build/

install: build
	install -Dm755 build/zdns $(DESTDIR)$(BINDIR)/zdns
	install -Dm644 zdns.service $(DESTDIR)/usr/lib/systemd/user/zdns.service

uninstall:
	rm -f $(DESTDIR)$(BINDIR)/zdns
	rm -f $(DESTDIR)/usr/lib/systemd/user/zdns.service
