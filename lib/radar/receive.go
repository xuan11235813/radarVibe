package radar

import (
	"context"
	"fmt"
	"net"
	"time"
)

// ReceiveFrame waits for a complete, verified frame. The caller owns conn.
// sourceIP optionally limits the radar IP. Once a valid frame-start packet
// arrives, its IP and port are pinned for this call to avoid mixing radars.
// Invalid or incomplete frames are discarded until the context expires.
func ReceiveFrame(ctx context.Context, conn *net.UDPConn, sourceIP net.IP) (*Frame, error) {
	var assembler Assembler
	var source *net.UDPAddr
	var lastRejected error
	packet := make([]byte, 65535)
	for {
		if err := ctx.Err(); err != nil {
			if lastRejected != nil {
				return nil, fmt.Errorf("receive frame (last rejected packet/frame: %v): %w", lastRejected, err)
			}
			return nil, err
		}
		deadline := time.Now().Add(200 * time.Millisecond)
		if end, ok := ctx.Deadline(); ok && end.Before(deadline) {
			deadline = end
		}
		if err := conn.SetReadDeadline(deadline); err != nil {
			return nil, fmt.Errorf("set UDP deadline: %w", err)
		}
		n, remote, err := conn.ReadFromUDP(packet)
		if err != nil {
			if e, ok := err.(net.Error); ok && e.Timeout() {
				continue
			}
			return nil, fmt.Errorf("read UDP: %w", err)
		}
		if sourceIP != nil && !sourceIP.Equal(remote.IP) {
			continue
		}
		if source != nil && (!source.IP.Equal(remote.IP) || source.Port != remote.Port) {
			continue
		}
		if source == nil {
			offset, err := PacketOffset(packet[:n])
			if err != nil || offset != 0 {
				continue
			}
			source = remote
		}
		frame, err := assembler.Add(packet[:n])
		if err != nil {
			lastRejected = err
			continue
		}
		if frame != nil {
			return frame, nil
		}
	}
}
