# 网络诊断（Network Diagnostics）

> 内置网络诊断工具，支持 Ping 和 Traceroute。

**相关文档**: [主机管理](host-management.md) | [服务配置](services.md)

---

页面路径：`/network`

内置网络诊断工具页面，两个 Tab：

## Ping

- 目标主机输入
- 参数配置：次数、包大小、TTL、间隔、超时、是否持续
- 流式输出：实时显示每个 ping 包的响应
- 统计摘要：发送/接收/丢失数、往返时间最小/平均/最大值
- 可选择出口接口

## Traceroute

- 目标主机输入
- 流式输出：逐跳显示路由路径
- 每跳显示：序号、主机名、IP、各探测时间
- 可选择出口接口
