// Command serve previews the site locally: go run ./tools/serve [-addr :8000] [dir]
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8000", "address to listen on")
	flag.Parse()
	dir := "."
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	}
	fmt.Printf("Serving %s at http://%s/ (Ctrl+C to stop)\n", dir, *addr)
	log.Fatal(http.ListenAndServe(*addr, http.FileServer(http.Dir(dir))))
}
