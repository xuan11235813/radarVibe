package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"radar/lib/radar"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "receiveData:", err)
		os.Exit(1)
	}
}

func run() error {
	listen := flag.String("listen", fmt.Sprintf("0.0.0.0:%d", radar.DefaultPort), "local IPv4 UDP listen address")
	output := flag.String("output", "", "CSV output path (default: timestamped file in current directory)")
	timeout := flag.Duration("timeout", 30*time.Second, "capture timeout; 0 waits indefinitely")
	source := flag.String("source", "", "optional radar IPv4 address")
	flag.Parse()
	if flag.NArg() != 0 || *timeout < 0 {
		return fmt.Errorf("unexpected arguments or negative timeout; use -h for help")
	}
	var sourceIP net.IP
	if *source != "" {
		sourceIP = net.ParseIP(*source)
		if sourceIP == nil || sourceIP.To4() == nil {
			return fmt.Errorf("invalid radar IPv4 address: %q", *source)
		}
	}
	addr, err := net.ResolveUDPAddr("udp4", *listen)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", *listen, err)
	}
	defer conn.Close()
	if err := conn.SetReadBuffer(16 * 1024 * 1024); err != nil {
		fmt.Fprintln(os.Stderr, "Warning: cannot enlarge UDP receive buffer:", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}
	fmt.Printf("Listening on UDP %s; waiting for one complete radar frame...\n", conn.LocalAddr())
	frame, err := radar.ReceiveFrame(ctx, conn, sourceIP)
	if err != nil {
		return err
	}
	// Stop receiving while writing the captured frame.
	_ = conn.Close()
	if *output == "" {
		*output = "radar_" + time.Now().Format("20060102_150405.000000000") + ".csv"
	}
	if err := radar.SaveCSV(*output, frame); err != nil {
		return fmt.Errorf("save CSV: %w", err)
	}
	path, err := filepath.Abs(*output)
	if err != nil {
		path = *output
	}
	fmt.Printf("Saved %d samples (ch6, ch7) to %s\n", radar.SamplesPerChannel, path)
	return nil
}
