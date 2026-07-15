---
name: build-output-location
description: Always compile Go binaries to project root directory
metadata:
  type: feedback
---

Output compiled Go binaries to the project root directory (D:\NewCB\pxelab\) or a bin/ subdirectory under it. Not to /tmp.

**Why:** User found /tmp output confusing on Windows and wants binaries in a predictable location.

**How to apply:** When compiling, use `-o ./pxelab.exe` (or `-o ./bin/pxelab.exe`) relative to the project root. Avoid /tmp, /var/tmp, or other system temp paths.
