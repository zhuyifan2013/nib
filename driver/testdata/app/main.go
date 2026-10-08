// Local fixture for direct-runtime and resident-shell driver verification.
package main

import (
	"encoding/json"
	"log"
	"nib.dev/nib/core"
	"nib.dev/nib/driver"
	"nib.dev/nib/ipc"
	"os"
	"strconv"
)

func main() {
	port, _ := strconv.Atoi(os.Getenv("NIB_TEST_DRIVER_PORT"))
	r := core.New(core.Options{Title: "Driver fixture", Width: 640, Height: 480, Headless: true, Driver: driver.Options{Port: port, Token: os.Getenv("NIB_DRIVER_TOKEN")}})
	r.Bind("Fixture.Echo", func(args json.RawMessage) (any, *ipc.Error) { return args, nil })
	r.Bind("Fixture.Fail", func(args json.RawMessage) (any, *ipc.Error) {
		return nil, ipc.NewError("fixture/failure", "intentional failure")
	})
	if err := r.Run(`<html><head><title>Fixture</title></head><body><input id="input"><button id="go" onclick="document.title=document.querySelector('#input').value">Go</button></body></html>`); err != nil {
		log.Fatal(err)
	}
}
