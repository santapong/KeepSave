// keepsave-connector is the reviewed, credential-free Unix relay client.
package main

import (
	"context"
	"flag"
	"os"
	"time"

	"github.com/santapong/KeepSave/backend/internal/runner"
)

func main() {
	request := flag.String("request", "/relay/request.json", "typed request file")
	socket := flag.String("socket", "/relay/execute.sock", "per-attempt Unix relay")
	flag.Parse()
	if flag.NArg() != 0 || *request != "/relay/request.json" || *socket != "/relay/execute.sock" {
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if runner.ExecuteConnector(ctx, *request, *socket, os.Stdout) != nil {
		os.Exit(1)
	}
}
