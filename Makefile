PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin

.PHONY: all build clean install uninstall test

all: build

build:
	go build -o zdns ./cmd/zdns

test:
	go test -v ./pkg/...

clean:
	rm -f zdns

install: build
	install -Dm755 zdns $(DESTDIR)$(BINDIR)/zdns
	install -Dm644 zdns.service $(DESTDIR)/usr/lib/systemd/user/zdns.service

uninstall:
	rm -f $(DESTDIR)$(BINDIR)/zdns
	rm -f $(DESTDIR)/usr/lib/systemd/user/zdns.service
