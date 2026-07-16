BIN := gitlab-tui
CMD := ./internal/gitlab/client.go
INSTALL_DIR := /usr/local/bin

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
