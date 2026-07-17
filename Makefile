BIN := gitlab-tui
CMD := ./cmd/gitlab-tui
PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin
INSTALL_DIR := $(DESTDIR)$(BINDIR)

.PHONY: build install run clean

build:
	go build -o $(BIN) $(CMD)

install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BIN) $(INSTALL_DIR)/$(BIN)
	@echo "Installed to $(INSTALL_DIR)/$(BIN)"

run:
	go run $(CMD)

clean:
	rm -f $(BIN)
