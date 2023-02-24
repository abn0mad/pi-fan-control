package main

// Simple Pi Fan Control
// Start / Stop fan according to temperature threshold

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/stianeikeland/go-rpio/v4"
)

func memUsage() {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	allocatedTotal := mem.TotalAlloc / 1024 / 1024
	allocated := mem.Alloc / 1024 / 1024
	allocatedBySystem := mem.Sys / 1024 / 1024
	log.Printf("Memory usage (allocated): %v\n", allocated)
	log.Printf("Memory usage (total allocated): %v\n", allocatedTotal)
	log.Printf("Memory usage (allocated by system): %v\n", allocatedBySystem)
}

func currentTemp(source string) (int, error) {
	rawTempUnformatted, err := ioutil.ReadFile(source)
	if err != nil {
		log.Fatal(err)
		return 0, err
	}
	rawTempFormatted := strings.Replace(string(rawTempUnformatted), "\n", "", -1)
	sysTemp, err := strconv.Atoi(string(rawTempFormatted))
	if err != nil {
		log.Fatal(err)
		return 0, err
	}
	humanReadable := sysTemp / 1000
	return humanReadable, nil
}

func fanOn(pin rpio.Pin) {
	pin.Write(1)
}

func fanOff(pin rpio.Pin) {
	pin.Write(0)
}

func pinState(pin rpio.Pin) int {
	state := pin.Read()
	return int(state)
}

func fanControl(start int, stop int, timeout int, thermal string, pin rpio.Pin) {
	for {
		cpuTemp, err := currentTemp(thermal)
		if err != nil {
			log.Fatal(err)
		}

		mode := os.Getenv("MODE")
		if mode == "debug" {
			memUsage()
			log.Printf("CPU temperature: %v\n", cpuTemp)
			log.Printf("GPIO pin state: %v\n", pinState(pin))
		}

		/*
			if cpuTemp <= stop {
				state := pinState(pin)
				if state == 1 {
					fanOff(pin)
				}
			} else {
				fanOn(pin)
			}
		*/

		if cpuTemp >= start {
			fanOn(pin)
		} else if cpuTemp <= stop {
			state := pinState(pin)
			if state == 1 {
				fanOff(pin)
			}
		}

		time.Sleep(time.Duration(timeout) * time.Second)
	}
}

func usage() {
	fmt.Print("\n")
	fmt.Printf("Usage of %s:\n", os.Args[0])
	fmt.Print("\n")
	fmt.Print("'-start' Temperature threshold (start)\n")
	fmt.Print("'-stop'  Temperature threshold (stop)\n")
	fmt.Print("'-timeout' Timeout in seconds\n")
	fmt.Print("'-thermal' Thermal information source\n")
	fmt.Print("'-gpio' GPIO pin\n")
	fmt.Print("\n")
	fmt.Print("Example:\n")
	fmt.Print("\n")
	fmt.Printf("'%s -start 68 -stop 60 -timeout 5 -thermal /sys/class/thermal/thermal_zone0/temp -gpio 2'", os.Args[0])
	fmt.Print("\n")
}

func main() {

	// register command line flags
	startFan := flag.Int("start", 68, "Temperature threshold (start)")
	stopFan := flag.Int("stop", 60, "Temperature threshold (stop)")
	timeout := flag.Int("timeout", 5, "Timeout in seconds")
	thermalInfo := flag.String("thermal", "/sys/class/thermal/thermal_zone0/temp", "Thermal information source")
	gpio := flag.Int("gpio", 2, "GPIO pin")
	// replace default usage message
	flag.Usage = usage
	// parse command line flags
	flag.Parse()

	// open GPIO mem
	if err := rpio.Open(); err != nil {
		log.Println(err)
		os.Exit(1)
	}

	// keep GPIO mem open until program end
	defer rpio.Close()

	// set GPIO pin
	pin := rpio.Pin(*gpio)
	pin.Output()

	// prepare channels, waitgroups and OS signal catches
	var sigCh = make(chan os.Signal, 1)
	signal.Notify(sigCh,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	// pre-exit goroutine
	go func() {
		sig := <-sigCh
		log.Printf("Caught signal: %+v\n", sig)
		log.Print("Stopping PiFan fan monitor...\n")
		fanOff(pin)
		rpio.Close()
		log.Print("PiFan fan monitor: stopped.\n")
		os.Exit(0)
	}()

	// prepare waitgroup
	var wg sync.WaitGroup

	// add group
	wg.Add(1)

	// main goroutine
	go func() {
		fanControl(*startFan, *stopFan, *timeout, *thermalInfo, pin)
		wg.Done()
	}()

	log.Print("PiFan fan monitor: running.")
	wg.Wait()
}
