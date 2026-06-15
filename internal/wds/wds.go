package wds

import "net/http"

// RegisterRoutes 注册 WDS 相关的 HTTP 路由
func RegisterRoutes(mux *http.ServeMux, rootDir string) {
	fs := http.FileServer(http.Dir(rootDir))
	mux.Handle("/boot/wds/", http.StripPrefix("/boot/wds/", fs))
}

/*
WDS 启动流程：
1. 客户端请求 bootmgr.exe → 从 boot/winpe/ 提供
2. 客户端请求 BCD → 返回预配置的 BCD 文件
3. 客户端请求 boot.sdi → 返回 boot.sdi
4. 客户端请求 boot.wim → 返回 Windows PE 映像
*/
