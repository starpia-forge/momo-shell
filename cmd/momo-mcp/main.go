// Command momo-mcp is a dumb stdio<->pipe byte tunnel for MCP clients
// (doc 20 D3): it does no MCP/JSON-RPC/SDK parsing. In its default (auth)
// mode it authenticates to the momo-shell GUI's mcpipc server with a token
// read from MOMO_MCP_TOKEN, then transparently pumps stdin<->pipe so an
// MCP host (e.g. Claude Desktop) speaks MCP directly to the GUI's SDK
// server (A6) over the connection. In --pair mode it instead requests a
// new token via human approval in the GUI and prints it to stdout.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"momo-shell/internal/adapter/in/mcpipc"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("momo-mcp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	pair := fs.Bool("pair", false, "request a new token via human approval in the momo-shell GUI, print it to stdout, and exit")
	name := fs.String("name", defaultClientName(), "client name shown in the momo-shell pairing approval prompt")
	if err := fs.Parse(args); err != nil {
		return 2 // flag already printed usage/error to stderr
	}

	conn, err := mcpipc.Dial()
	if err != nil {
		fmt.Fprintf(stderr, "momo-mcp: momo-shell이 실행 중이 아니거나 연결할 수 없습니다: %v\n", err)
		return 1
	}
	defer conn.Close()

	if *pair {
		token, err := mcpipc.Pair(conn, *name)
		if err != nil {
			fmt.Fprintf(stderr, "momo-mcp: pairing 실패: %v\n", err)
			return 1
		}
		fmt.Fprintln(stderr, "momo-mcp: pairing 승인됨. 아래 토큰을 MOMO_MCP_TOKEN 환경변수로 저장하세요.")
		fmt.Fprintln(stdout, token)
		return 0
	}

	token := os.Getenv("MOMO_MCP_TOKEN")
	if token == "" {
		fmt.Fprintln(stderr, "momo-mcp: MOMO_MCP_TOKEN 환경변수가 설정되어 있지 않습니다.")
		return 1
	}

	if err := mcpipc.Authenticate(conn, token); err != nil {
		fmt.Fprintf(stderr, "momo-mcp: 인증 실패: %v\n", err)
		return 1
	}

	return pump(conn, stdin, stdout)
}

func defaultClientName() string {
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return "momo-mcp"
}

// pump transparently copies bytes in both directions between conn and the
// MCP host's stdin/stdout (doc 20 D3: no MCP/JSON-RPC parsing here). It
// returns once either direction hits EOF/error, closing conn so the other
// direction and the server observe the shutdown instead of hanging.
func pump(conn io.ReadWriteCloser, stdin io.Reader, stdout io.Writer) int {
	done := make(chan error, 2)
	go func() {
		_, err := io.Copy(conn, stdin)
		done <- err
	}()
	go func() {
		_, err := io.Copy(stdout, conn)
		done <- err
	}()

	err := <-done
	conn.Close()
	if err != nil {
		return 1
	}
	return 0
}
