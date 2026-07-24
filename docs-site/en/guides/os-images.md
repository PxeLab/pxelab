# OS Images

> ISO image upload, mount, extraction, and file browsing.

**Docs**: [Netboot Catalog](netboot.md) | [File Management](web-ui.md) | [REST API Reference](../reference/api-reference.md)

---

## API Endpoints

```
GET    /api/v1/os-images                # List all images
POST   /api/v1/os-images/upload         # Upload ISO
POST   /api/v1/os-images/import         # Import local file
GET    /api/v1/os-images/{id}           # Get image details
PUT    /api/v1/os-images/{id}           # Update metadata
DELETE /api/v1/os-images/{id}           # Delete image
POST   /api/v1/os-images/{id}/extract   # Extract ISO
POST   /api/v1/os-images/{id}/mount     # Mount ISO
POST   /api/v1/os-images/{id}/unmount   # Unmount ISO
POST   /api/v1/os-images/{id}/reprocess # Reprocess
GET    /api/v1/os-images/{id}/file      # Download file
```

---

## Features

- **Upload ISO** — Drag-and-drop or file picker upload
- **Import Local File** — Import from server local path
- **Auto-detection** — Automatically identify distro type
- **Mount/Unmount** — ISO mount point management
- **File Browsing** — Browse mounted ISO contents
- **Download** — Download image files
- **Reprocess** — Re-detect and re-parse images
