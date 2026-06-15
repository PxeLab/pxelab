package api

import (
	"net"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/pxego/pxego/internal/store"
)

type WOLHandler struct {
	store store.Interface
}

func (h *WOLHandler) Wake(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	host, err := h.store.GetHost(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "主机未找到")
		return
	}

	mac, err := net.ParseMAC(host.MAC)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的 MAC 地址")
		return
	}

	// 构建 WOL 魔术包：6 字节 0xFF + 16 次重复的 MAC 地址
	packet := make([]byte, 0, 102)
	packet = append(packet, []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}...)
	for i := 0; i < 16; i++ {
		packet = append(packet, mac...)
	}

	// 通过 UDP 广播发送
	addr, err := net.ResolveUDPAddr("udp4", "255.255.255.255:9")
	if err != nil {
		Error(w, http.StatusInternalServerError, "地址解析失败")
		return
	}
	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		Error(w, http.StatusInternalServerError, "UDP 连接失败")
		return
	}
	defer conn.Close()

	if _, err := conn.Write(packet); err != nil {
		Error(w, http.StatusInternalServerError, "发送唤醒包失败")
		return
	}

	OK(w, map[string]string{"message": "WOL 魔术包已发送"})
}
