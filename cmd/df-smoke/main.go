// Command df-smoke is a manual test client for the DFHack plugin. It
// starts the TCP listener, waits for the plugin to connect (run
// `ai-connect` in the DFHack console), sends exactly one command or
// query, prints the result, and exits.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/df-ai/orchestrator/internal/commands"
	"github.com/df-ai/orchestrator/internal/dfhack"
	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/df-ai/orchestrator/internal/protocol"
)

var (
	port     = flag.Uint("port", 5000, "TCP port to listen on (must match plugin)")
	cmd      = flag.String("cmd", "", "dig|build|order|query")
	digtype  = flag.String("digtype", "default", "default|stairs|channel|ramp|upstair|downstair")
	buildtyp = flag.Uint("buildtype", 0x10, "protocol BuildType byte (0x10=carpenter)")
	ordertyp = flag.Uint("ordertype", 1, "protocol OrderType byte (1=bed)")
	qty      = flag.Uint("qty", 1, "work order quantity")
	qname    = flag.String("name", "sim_status", "query name")
	qargs    = flag.String("args", "{}", "query args JSON")
	x1       = flag.Int("x1", 0, "")
	y1       = flag.Int("y1", 0, "")
	z1       = flag.Int("z1", 0, "")
	x2       = flag.Int("x2", 0, "")
	y2       = flag.Int("y2", 0, "")
	z2       = flag.Int("z2", 0, "")
	waitSecs = flag.Int("wait", 120, "seconds to wait for plugin connection")
)

func digTypeByte(s string) uint8 {
	switch s {
	case "stairs":
		return protocol.DigTypeUpDownStair
	case "channel":
		return protocol.DigTypeChannel
	case "ramp":
		return protocol.DigTypeRamp
	case "downstair":
		return protocol.DigTypeDownStair
	case "upstair":
		return protocol.DigTypeUpStair
	default:
		return protocol.DigTypeDefault
	}
}

func main() {
	flag.Parse()
	logger := logging.NewTextLogger("info")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := dfhack.NewClient(logger)
	if err := client.Start(ctx, uint16(*port)); err != nil {
		fmt.Fprintf(os.Stderr, "listen failed: %v\n", err)
		os.Exit(1)
	}
	exec := commands.NewCommandExecutor(logger, client, 30*time.Second)

	fmt.Printf("listening on :%d — run `ai-connect` in the DFHack console\n", *port)
	deadline := time.Now().Add(time.Duration(*waitSecs) * time.Second)
	for !client.IsConnected() {
		if time.Now().After(deadline) {
			fmt.Fprintln(os.Stderr, "plugin never connected")
			os.Exit(1)
		}
		time.Sleep(500 * time.Millisecond)
	}
	// Give the FULL_STATE handshake a moment to finish before commanding.
	time.Sleep(2 * time.Second)

	var (
		res *commands.CommandResult
		err error
	)
	switch *cmd {
	case "dig":
		res, err = exec.SendDigRegion(digTypeByte(*digtype),
			int16(*x1), int16(*y1), int16(*z1), int16(*x2), int16(*y2), int16(*z2))
	case "build":
		res, err = exec.SendBuildCommand(int16(*x1), int16(*y1), int16(*z1), uint8(*buildtyp))
	case "order":
		res, err = exec.SendWorkOrderCommand(uint8(*ordertyp), uint16(*qty))
	case "query":
		var raw []byte
		raw, err = client.SendQuery(ctx, *qname, *qargs, 10*time.Second)
		if err == nil {
			fmt.Printf("QUERY OK: %s\n", string(raw))
		}
	default:
		fmt.Fprintln(os.Stderr, "unknown -cmd (dig|build|order|query)")
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
	if res != nil {
		fmt.Printf("ACK: success=%v status=%d error=%q duration=%s\n",
			res.Success, res.Status, res.ErrorMsg, res.Duration)
	}
	_ = client.Stop()
	exec.Stop()
}
