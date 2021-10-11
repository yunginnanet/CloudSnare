package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/registrobr/rdap"
	"github.com/registrobr/rdap/protocol"
)

var ips []net.IP
var count = 0

func init() {
	if len(os.Args) < 1 {
		println("need file")
		os.Exit(1)
	}

	f, err := os.Open(os.Args[1])
	if err != nil {
		os.Exit(1)
	}

	scan := bufio.NewScanner(f)

	for scan.Scan() {
		if ip := net.ParseIP(scan.Text()); ip == nil {
			println("invalid: " + scan.Text())
		} else {
			ips = append(ips, ip)
		}
	}
}

var cloudflares []net.IP

func rdapit(addr net.IP) (err error) {
	mu.Lock()
	working++
	mu.Unlock()
	defer func() {
		mu.Lock()
		working--
		mu.Unlock()
	}()
	c := rdap.NewClient([]string{"http://rdap-bootstrap.arin.net/bootstrap/"})
	var d *protocol.IPNetwork

	d, _, err = c.IP(addr, nil, nil)
	if err != nil {
		return err
	}

	if strings.Contains(strings.ToLower(d.Name), "cloudfla") {
		println(addr.String())
		cloudflares = append(cloudflares, addr)
	}

	return nil
}

var working = 0
var maxWorkers = 20
var mu = &sync.RWMutex{}
var checked = make(map[string]uint8)
var checking = make(map[string]uint8)
var tocheck = make(chan net.IP, 100)


var start = make(chan uint8)
var quit = make(chan uint8)


func main() {
	go listen()
	for _, ip := range ips {
		go func() {
			count++
			tocheck <- ip
		}()
	}
	fmt.Printf("loaded %d ips\n", count)
	start <- 0
	<-quit
	println("fin")

}

func listen() {
	for {
		select {
		case ipa := <-tocheck:
			time.Sleep(time.Duration(10*failcount) * time.Millisecond)
			for working >= maxWorkers {
				time.Sleep(250 * time.Millisecond)
				// fmt.Printf(".")
			}
			mu.RLock()
			if _, ok := checked[ipa.String()]; ok {
				mu.RUnlock()
				continue
			}
			mu.RUnlock()
			go worker(ipa)
		default:
			if len(checked) == count {
				quit <- 0
			}
		}
	}
}

var failcount = 0

func worker(ip net.IP) {

	mu.RLock()
	if _, ok := checking[ip.String()]; ok {
		mu.RUnlock()
		return
	}
	mu.RUnlock()

	mu.Lock()
	checking[ip.String()] = 0
	mu.Unlock()

	defer func() {
		mu.Lock()
		delete(checking, ip.String())
		mu.Unlock()
	}()

	err := rdapit(ip)
	if err != nil {
		failcount++
		// print("f")
		// fmt.Println(err.Error())
		tocheck <- ip
	}
	mu.Lock()
	checked[ip.String()] = 0
	mu.Unlock()

}
