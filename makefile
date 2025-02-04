.PHONY: FORCE

OBJ := thirteen gopher-ls

build: $(OBJ)

%: cmd/% cmd/%/*.go
	go build ./$<

test: FORCE
	go test -v ./...

clean: FORCE
	rm -f $(OBJ)

PREFIX = /usr/local

install: build FORCE
	mkdir -p $(PREFIX)/sbin $(PREFIX)/bin $(PREFIX)/share/man/man8
	cp thirteen  $(PREFIX)/sbin
	cp gopher-ls $(PREFIX)/bin
	cp doc/thirteen.8 $(PREFIX)/share/man/man8
