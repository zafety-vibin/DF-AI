// Command df-smoke is a manual test client for the DFHack plugin. It
// starts the TCP listener, waits for the plugin to connect (run
// `ai-connect` in the DFHack console), sends exactly one command or
// query, prints the result, and exits.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/df-ai/orchestrator/internal/commands"
	"github.com/df-ai/orchestrator/internal/dfhack"
	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/df-ai/orchestrator/internal/protocol"
)

var (
	port     = flag.Uint("port", 5000, "TCP port to listen on (must match plugin)")
	cmd      = flag.String("cmd", "", "dig|build|order|query|pause|unpause|step")
	digtype  = flag.String("digtype", "default", "default|stairs|channel|ramp|upstair|downstair")
	buildtyp = flag.Uint("buildtype", 0x10, "protocol BuildType byte (0x10=carpenter)")
	ordertyp = flag.Uint("ordertype", 1, "protocol OrderType byte (1=bed)")
	qty      = flag.Uint("qty", 1, "work order quantity / step tick count")
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
	case "pause":
		res, err = exec.SendPauseCommand(true)
	case "unpause":
		res, err = exec.SendPauseCommand(false)
	case "step":
		res, err = exec.SendStepCommand(uint32(*qty))
	case "repl":
		runREPL(ctx, client, exec)
	default:
		fmt.Fprintln(os.Stderr, "unknown -cmd (dig|build|order|query|pause|unpause|step|repl)")
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

// runREPL keeps ONE plugin connection alive across many commands — used at
// live checkpoints so the user doesn't re-run ai-connect per test.
// Commands: dig <type> <x1> <y1> <z1> <x2> <y2> <z2> | build <typebyte> <x> <y> <z>
//
//	order <typebyte> <qty> | query <name> [argsJSON] | pause | unpause
//	step <ticks> | quit
func runREPL(ctx context.Context, client *dfhack.Client, exec *commands.CommandExecutor) {
	sc := bufio.NewScanner(os.Stdin)
	fmt.Println("REPL ready (dig/build/order/query/pause/unpause/step/quit)")
	for {
		fmt.Print("> ")
		if !sc.Scan() {
			return
		}
		f := strings.Fields(sc.Text())
		if len(f) == 0 {
			continue
		}
		n := func(i int) int16 { v, _ := strconv.Atoi(f[i]); return int16(v) }
		var (
			res *commands.CommandResult
			err error
		)
		switch f[0] {
		case "quit", "exit":
			return
		case "dig":
			if len(f) < 8 {
				fmt.Println("usage: dig <type> <x1> <y1> <z1> <x2> <y2> <z2>")
				continue
			}
			res, err = exec.SendDigRegion(digTypeByte(f[1]), n(2), n(3), n(4), n(5), n(6), n(7))
		case "build":
			if len(f) < 5 {
				fmt.Println("usage: build <typebyte> <x> <y> <z>")
				continue
			}
			bt, _ := strconv.ParseUint(strings.TrimPrefix(f[1], "0x"), 16, 8)
			res, err = exec.SendBuildCommand(n(2), n(3), n(4), uint8(bt))
		case "order":
			if len(f) < 3 {
				fmt.Println("usage: order <typebyte> <qty>")
				continue
			}
			ot, _ := strconv.ParseUint(strings.TrimPrefix(f[1], "0x"), 16, 8)
			q, _ := strconv.Atoi(f[2])
			res, err = exec.SendWorkOrderCommand(uint8(ot), uint16(q))
		case "query":
			if len(f) < 2 {
				fmt.Println("usage: query <name> [argsJSON]")
				continue
			}
			args := "{}"
			if len(f) > 2 {
				args = strings.Join(f[2:], " ")
			}
			var raw []byte
			raw, err = client.SendQuery(ctx, f[1], args, 10*time.Second)
			if err == nil {
				fmt.Printf("QUERY OK: %s\n", string(raw))
			}
		case "pause":
			res, err = exec.SendPauseCommand(true)
		case "unpause":
			res, err = exec.SendPauseCommand(false)
		case "step":
			if len(f) < 2 {
				fmt.Println("usage: step <ticks>")
				continue
			}
			t, _ := strconv.Atoi(f[1])
			res, err = exec.SendStepCommand(uint32(t))
		default:
			fmt.Println("unknown command")
			continue
		}
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			continue
		}
		if res != nil {
			fmt.Printf("ACK: success=%v status=%d error=%q duration=%s\n",
				res.Success, res.Status, res.ErrorMsg, res.Duration)
		}
	}
}
