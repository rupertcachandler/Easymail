.PHONY: build dev clean install deb frontend-build frontend-dev bindings deps

BINARY=easymail
VERSION=0.1.0
ARCH=amd64
BUILD_DIR=build
FRONTEND_DIR=frontend
GO?=go
LDFLAGS=-s -w

all: build

## build: Build the production binary
build: frontend-build
	CGO_ENABLED=1 GOTOOLCHAIN=auto $(GO) build -tags "webkit2_41 production" -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) .

## dev: Start Wails dev mode with hot reload
dev:
	$(HOME)/go/bin/wails dev -tags "webkit2_41 production"

## frontend-build: Build the Vue frontend
frontend-build:
	cd $(FRONTEND_DIR) && npm run build

## frontend-dev: Start the Vite dev server
frontend-dev:
	cd $(FRONTEND_DIR) && npm run dev

## clean: Remove build artifacts
clean:
	rm -rf $(BUILD_DIR)/$(BINARY)
	rm -rf $(FRONTEND_DIR)/dist
	rm -rf $(FRONTEND_DIR)/node_modules

## install: Install to /usr/local/bin
install: build
	cp $(BUILD_DIR)/$(BINARY) /usr/local/bin/

## deb: Build a Debian package
deb: build
	@rm -rf /tmp/easymail-$(VERSION)_deb
	@mkdir -p /tmp/easymail-$(VERSION)_deb/DEBIAN
	@mkdir -p /tmp/easymail-$(VERSION)_deb/usr/bin
	@mkdir -p /tmp/easymail-$(VERSION)_deb/usr/share/applications
	@mkdir -p /tmp/easymail-$(VERSION)_deb/usr/share/icons/hicolor/scalable/apps
	@mkdir -p /tmp/easymail-$(VERSION)_deb/usr/share/doc/easymail
	cp $(BUILD_DIR)/$(BINARY) /tmp/easymail-$(VERSION)_deb/usr/bin/easymail
	strip /tmp/easymail-$(VERSION)_deb/usr/bin/easymail
	cp frontend/easymail.svg /tmp/easymail-$(VERSION)_deb/usr/share/icons/hicolor/scalable/apps/easymail.svg
	cp packaging/easymail.desktop /tmp/easymail-$(VERSION)_deb/usr/share/applications/easymail.desktop
	cp packaging/control /tmp/easymail-$(VERSION)_deb/DEBIAN/control
	cp packaging/postinst /tmp/easymail-$(VERSION)_deb/DEBIAN/postinst
	gzip -9cn packaging/changelog > /tmp/easymail-$(VERSION)_deb/usr/share/doc/easymail/changelog.Debian.gz
	chmod 755 /tmp/easymail-$(VERSION)_deb/DEBIAN/postinst
	dpkg-deb --build /tmp/easymail-$(VERSION)_deb easymail_$(VERSION)_$(ARCH).deb
	@rm -rf /tmp/easymail-$(VERSION)_deb
	@echo "Built: easymail_$(VERSION)_$(ARCH).deb"

## bindings: Regenerate Wails JS bindings
bindings:
	$(HOME)/go/bin/wails generate module

## deps: Install Go and Node dependencies
deps:
	cd $(FRONTEND_DIR) && npm install
	$(GO) mod download