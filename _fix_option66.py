import re

with open('internal/dhcp/handler.go', 'r', encoding='utf-8') as f:
    content = f.read()

# Replace the option 43 comment/block to add option 66 before it
old = """\t\t// Option 43 · PXE Vendor Specific — Discovery Control
\t\t// 子选项 6 值 0x0C = 禁用 PXE 服务器发现(bit2) + 禁止使用 DHCP 启动文件(bit3)
\t\t// 注意：bit3=1 看似矛盾（不用 DHCP 中的启动文件），但实测 0x04 导致 PXE-E76
\t\treply.UpdateOption(dhcpv4.OptGeneric(dhcpv4.OptionVendorSpecificInformation,
\t\t\t[]byte{6, 1, 0x0C}))"""

new = """\t\t// Option 66 — TFTP Server Name (PXE_STACK 通过此选项传递代理服务器 IP)
\t\tif nextServer != nil {
\t\t\treply.UpdateOption(dhcpv4.OptGeneric(dhcpv4.OptionTFTPServerName,
\t\t\t\t[]byte(nextServer.String())))
\t\t}

\t\t// Option 43 · PXE Vendor Specific — Discovery Control
\t\t// 子选项 6 值 0x0C = 禁用 PXE 服务器发现(bit2) + 禁止使用 DHCP 启动文件(bit3)
\t\t// 注意：bit3=1 看似矛盾（不用 DHCP 中的启动文件），但实测 0x04 导致 PXE-E76
\t\treply.UpdateOption(dhcpv4.OptGeneric(dhcpv4.OptionVendorSpecificInformation,
\t\t\t[]byte{6, 1, 0x0C}))"""

if old in content:
    content = content.replace(old, new, 1)
    with open('internal/dhcp/handler.go', 'w', encoding='utf-8') as f:
        f.write(content)
    print('OK: option 66 added')
else:
    print('ERR: old string not found')
    # Debug: find context
    idx = content.find('PXE Vendor Specific')
    if idx >= 0:
        snippet = content[idx-5:idx+300]
        print('Found at', idx)
        print(repr(snippet))
    else:
        print('Option PXE Vendor Specific not found at all')
        idx2 = content.find('Discovery Control')
        if idx2 >= 0:
            print('Found Discovery Control at', idx2)
            print(repr(content[idx2-5:idx2+300]))
