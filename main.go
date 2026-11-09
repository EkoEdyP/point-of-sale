package main

import (
	"flag"

	"github.com/EkoEdyP/point-of-sale/delivery"
)

func main() {
	openBrowser := flag.Bool("buka", false, "buka aplikasi di browser setelah server siap (dipakai start.bat)")
	flag.Parse()

	server := delivery.NewServer()
	server.OpenBrowser = *openBrowser
	server.Run()
}
