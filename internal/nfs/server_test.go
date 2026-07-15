package nfs

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func TestBuildProgMismatchReply(t *testing.T) {
	xid := uint32(42)
	low := uint32(3)
	high := uint32(3)

	reply := buildProgMismatchReply(xid, low, high)

	if len(reply) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(reply))
	}

	if got := binary.BigEndian.Uint32(reply[0:4]); got != xid {
		t.Errorf("xid: expected %d, got %d", xid, got)
	}
	if got := binary.BigEndian.Uint32(reply[4:8]); got != 1 {
		t.Errorf("msg_type: expected 1 (REPLY), got %d", got)
	}
	if got := binary.BigEndian.Uint32(reply[8:12]); got != 0 {
		t.Errorf("reply_stat: expected 0 (MSG_ACCEPTED), got %d", got)
	}
	if got := binary.BigEndian.Uint32(reply[12:16]); got != 0 {
		t.Errorf("verf flavor: expected 0 (AUTH_NONE), got %d", got)
	}
	if got := binary.BigEndian.Uint32(reply[16:20]); got != 0 {
		t.Errorf("verf length: expected 0, got %d", got)
	}
	if got := binary.BigEndian.Uint32(reply[20:24]); got != 2 {
		t.Errorf("accept_stat: expected 2 (PROG_MISMATCH), got %d", got)
	}
	if got := binary.BigEndian.Uint32(reply[24:28]); got != low {
		t.Errorf("low: expected %d, got %d", low, got)
	}
	if got := binary.BigEndian.Uint32(reply[28:32]); got != high {
		t.Errorf("high: expected %d, got %d", high, got)
	}
}

func TestVersionAwareListener_RejectsNFSv4(t *testing.T) {
	rawLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer rawLn.Close()

	vl := &versionAwareListener{Listener: rawLn}

	// Start accepting in background
	done := make(chan struct{})
	go func() {
		_, _ = vl.Accept()
		close(done)
	}()

	// Connect and send NFSv4 RPC call (prog=100003, vers=4)
	conn, err := net.Dial("tcp", rawLn.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	var call bytes.Buffer
	binary.Write(&call, binary.BigEndian, uint32(123))    // xid
	binary.Write(&call, binary.BigEndian, uint32(0))      // msg_type = CALL
	binary.Write(&call, binary.BigEndian, uint32(2))      // rpcvers
	binary.Write(&call, binary.BigEndian, uint32(100003)) // prog (NFS)
	binary.Write(&call, binary.BigEndian, uint32(4))      // vers (v4)
	binary.Write(&call, binary.BigEndian, uint32(0))      // proc (any)

	callData := call.Bytes()
	marker := make([]byte, 4)
	binary.BigEndian.PutUint32(marker, uint32(len(callData))|(1<<31))
	conn.Write(append(marker, callData...))

	// Read response marker
	respMarker := make([]byte, 4)
	if _, err := conn.Read(respMarker); err != nil {
		t.Fatalf("reading response marker: %v", err)
	}
	respLen := binary.BigEndian.Uint32(respMarker) & ^uint32(1<<31)
	resp := make([]byte, respLen)
	if _, err := conn.Read(resp); err != nil {
		t.Fatalf("reading response body: %v", err)
	}

	// Verify PROG_MISMATCH
	if got := binary.BigEndian.Uint32(resp[20:24]); got != 2 {
		t.Errorf("accept_stat: expected 2 (PROG_MISMATCH), got %d", got)
	}
	if got := binary.BigEndian.Uint32(resp[24:28]); got != 3 {
		t.Errorf("low: expected 3, got %d", got)
	}
	if got := binary.BigEndian.Uint32(resp[28:32]); got != 3 {
		t.Errorf("high: expected 3, got %d", got)
	}

	// Close the raw listener to unblock the Accept loop
	rawLn.Close()
	<-done
}

func TestVersionAwareListener_PassesNFSv3(t *testing.T) {
	rawLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer rawLn.Close()

	vl := &versionAwareListener{Listener: rawLn}

	accepting := make(chan struct{})
	accepted := make(chan net.Conn, 1)
	go func() {
		close(accepting)
		conn, err := vl.Accept()
		if err != nil {
			return
		}
		accepted <- conn
	}()
	<-accepting

	conn, err := net.Dial("tcp", rawLn.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Send NFSv3 RPC call
	var call bytes.Buffer
	binary.Write(&call, binary.BigEndian, uint32(456))    // xid
	binary.Write(&call, binary.BigEndian, uint32(0))      // msg_type = CALL
	binary.Write(&call, binary.BigEndian, uint32(2))      // rpcvers
	binary.Write(&call, binary.BigEndian, uint32(100003)) // prog (NFS)
	binary.Write(&call, binary.BigEndian, uint32(3))      // vers (v3)
	binary.Write(&call, binary.BigEndian, uint32(0))      // proc (any)

	callData := call.Bytes()
	marker := make([]byte, 4)
	binary.BigEndian.PutUint32(marker, uint32(len(callData))|(1<<31))
	conn.Write(append(marker, callData...))

	select {
	case c := <-accepted:
		if c == nil {
			t.Fatal("nil connection accepted")
		}
		// Verify buffered data (28 bytes) is intact
		buf := make([]byte, 28)
		if _, err := c.Read(buf); err != nil {
			t.Fatalf("reading buffered data: %v", err)
		}
		// Check vers field in the RPC header
		if got := binary.BigEndian.Uint32(buf[20:24]); got != 3 {
			t.Errorf("vers: expected 3, got %d", got)
		}
		c.Close()
	case <-time.After(time.Second):
		t.Fatal("expected connection to be passed through for NFSv3")
	}
}

func TestBufferedConn(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	bufData := []byte{1, 2, 3, 4, 5}
	bc := &bufferedConn{Conn: server, buf: bufData}

	// First read returns buffered data
	first := make([]byte, 3)
	n, err := bc.Read(first)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("expected 3 bytes, got %d", n)
	}
	if !bytes.Equal(first, []byte{1, 2, 3}) {
		t.Errorf("expected [1,2,3], got %v", first)
	}

	// Second read returns remaining buffered data
	second := make([]byte, 5)
	n, err = bc.Read(second)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("expected 2 bytes, got %d", n)
	}
	if !bytes.Equal(second[:2], []byte{4, 5}) {
		t.Errorf("expected [4,5], got %v", second[:2])
	}

	// Third read reads from the underlying conn
	written := make(chan struct{})
	go func() {
		client.Write([]byte{42, 43})
		close(written)
	}()

	third := make([]byte, 2)
	n, err = bc.Read(third)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("expected 2 bytes, got %d", n)
	}
	if !bytes.Equal(third, []byte{42, 43}) {
		t.Errorf("expected [42,43], got %v", third)
	}
	<-written
}
