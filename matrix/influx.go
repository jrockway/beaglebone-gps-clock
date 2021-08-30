package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/net/trace"
)

var (
	influxEventLog = trace.NewEventLog("destination", "influxdb.jrock.us")
	token          string
)

func init() {
	token = os.Getenv("INFLUXDB_TOKEN")
	if token == "" {
		log.Println("not sending to influxdb; $INFLUXDB_TOKEN not set")
		influxEventLog.Errorf("not sent; INFLUXDB_TOKEN is empty")
	}
}

// We write our own InfluxDB client because the official one requires more memory to compile than
// the Beaglebone has.  I did this to avoid cross-compiling but honestly upgrading to go 1.17 made
// compiling on-device too slow, so this could all be removed now.

// sendToInflux writes "line protocol" data to InfluxDB.  If $INFLUXDB_TOKEN is empty, we print the
// line instead of sending it to the database.
func sendToInflux(body string) error {
	influxEventLog.Printf("%s", body)
	if token == "" {
		return nil
	}

	ctx, c := context.WithTimeout(context.Background(), 5*time.Second)
	defer c()
	req, err := http.NewRequestWithContext(ctx, "POST", "https://influxdb.jrock.us/api/v2/write?org=jrock.us&bucket=home-sensors", strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %v", err)
	}
	req.Header.Add("authorization", "Token "+token)
	req.Header.Add("content-type", "text/plain")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("make request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(res.Body)
		influxEventLog.Errorf("unexpected status %v", res.StatusCode)
		return fmt.Errorf("make request: unexpected status %v (%s): (body: %s)", res.StatusCode, res.Status, body)
	}
	return nil
}
