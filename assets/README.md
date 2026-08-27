# Boot assets (populated by CI via scripts/build-ipxe-assets.sh)

Place the following files here for ISO export:

- `ipxe.lkrn`
- `ipxe.efi` (built with EMBED chaining `file:/embed.ipxe`)
- `isolinux.bin` / `ldlinux.c32` (optional, enables BIOS boot)

GitHub Actions Release workflow builds these and packs them into each download archive.
