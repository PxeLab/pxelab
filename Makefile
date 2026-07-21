.PHONY: build test run clean frontend ipxe-build ipxe-build-embed ipxe-build-all ipxe-clean

# Go application build
build:
	go build -o bin/pxelab ./cmd/pxelab

test:
	go test ./...

run:
	go run ./cmd/pxelab

clean:
	rm -rf bin/  dist/

frontend:
	cd web && npm ci && npm run build

release:
	goreleaser release --clean

release-snapshot:
	goreleaser release --clean --snapshot

# iPXE build targets
# Non-embedded: relies on DHCP Option 175 + autoexec.ipxe (default)
# Embedded:     script compiled into binary (failsafe)
IPXE_VERSION ?= v2.0.0
IPXE_SRC_DIR ?= .ipxe-src
IPXE_EMBED   ?= boot/embedd.ipxe

# Clone/update iPXE source
ipxe-clone:
	@if [ ! -d "$(IPXE_SRC_DIR)" ]; then \
		echo "Cloning iPXE $(IPXE_VERSION)..."; \
		git clone --depth 1 --branch $(IPXE_VERSION) https://github.com/ipxe/ipxe.git $(IPXE_SRC_DIR); \
	else \
		echo "iPXE source already exists at $(IPXE_SRC_DIR)"; \
	fi

# Build x86_64 EFI — non-embedded (default, uses DHCP Option 175 + autoexec.ipxe)
ipxe-build: ipxe-clone
	cd $(IPXE_SRC_DIR)/src && make \
		bin-x86_64-efi/ipxe.efi \
		bin-x86_64-efi/snponly.efi
	cp $(IPXE_SRC_DIR)/src/bin-x86_64-efi/ipxe.efi boot/ipxe.efi
	cp $(IPXE_SRC_DIR)/src/bin-x86_64-efi/snponly.efi boot/ipxe-snponly.efi
	@echo "Built x86_64 EFI iPXE binaries (non-embedded) in boot/"

# Build x86_64 EFI — embedded (failsafe, script compiled in)
ipxe-build-embed: ipxe-clone
	cd $(IPXE_SRC_DIR)/src && make EMBED=../../$(IPXE_EMBED) \
		bin-x86_64-efi/ipxe.efi \
		bin-x86_64-efi/snponly.efi
	mkdir -p boot/embed
	cp $(IPXE_SRC_DIR)/src/bin-x86_64-efi/ipxe.efi boot/embed/ipxe.efi
	cp $(IPXE_SRC_DIR)/src/bin-x86_64-efi/snponly.efi boot/embed/ipxe-snponly.efi
	@echo "Built x86_64 EFI iPXE binaries (embedded) in boot/embed/"

# Build all architectures using Docker (both embedded and non-embedded)
ipxe-build-all: ipxe-clone
	@echo "=== Building non-embedded (default) ==="
	@for arch in arm32 arm64 i386 loong64 riscv32 riscv64 x86_64; do \
		echo "--- $$arch ---"; \
		docker run --rm -v $(PWD):/work -w /work \
			ghcr.io/ipxe/ipxe-builder-$$arch:latest \
			bash -c "cd $(IPXE_SRC_DIR)/src && make \
				bin-$$arch-efi/ipxe.efi \
				bin-$$arch-efi/snponly.efi" && \
		cp $(IPXE_SRC_DIR)/src/bin-$$arch-efi/ipxe.efi boot/ipxe-$$arch.efi && \
		cp $(IPXE_SRC_DIR)/src/bin-$$arch-efi/snponly.efi boot/snponly-$$arch.efi || \
		echo "Warning: Failed to build $$arch (non-embedded)"; \
	done
	@echo ""
	@echo "=== Building embedded (failsafe) ==="
	@mkdir -p boot/embed
	@for arch in arm32 arm64 i386 loong64 riscv32 riscv64 x86_64; do \
		echo "--- $$arch (embed) ---"; \
		docker run --rm -v $(PWD):/work -w /work \
			ghcr.io/ipxe/ipxe-builder-$$arch:latest \
			bash -c "cd $(IPXE_SRC_DIR)/src && make EMBED=../../$(IPXE_EMBED) \
				bin-$$arch-efi/ipxe.efi \
				bin-$$arch-efi/snponly.efi" && \
		cp $(IPXE_SRC_DIR)/src/bin-$$arch-efi/ipxe.efi boot/embed/ipxe-$$arch.efi && \
		cp $(IPXE_SRC_DIR)/src/bin-$$arch-efi/snponly.efi boot/embed/snponly-$$arch.efi || \
		echo "Warning: Failed to build $$arch (embedded)"; \
	done
	@echo "Build complete. boot/ = non-embedded, boot/embed/ = embedded."

# Clean iPXE build artifacts
ipxe-clean:
	rm -rf $(IPXE_SRC_DIR)
	@echo "Cleaned iPXE source directory"
