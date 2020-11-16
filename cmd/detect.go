package cmd

import (
	"net"
	"sync"
	"time"

	"github.com/andig/evcc/cmd/detect"
	"github.com/andig/evcc/hems/semp"
	"github.com/andig/evcc/util"
	"github.com/cheggaaa/pb/v3"
	"github.com/korylprince/ipnetgen"
	"github.com/spf13/cobra"
)

// detectCmd represents the vehicle command
var detectCmd = &cobra.Command{
	Use:   "detect",
	Short: "Detect compatible hardware",
	Run:   runDetect,
}

func init() {
	rootCmd.AddCommand(detectCmd)
}

// type Detector int

// const (
// 	timeout = 100 * time.Millisecond

// 	_ Detector = iota
// 	Ping
// 	Tcp
// 	Modbus
// 	Mqtt
// 	Http
// )

var (
	taskList = &detect.TaskList{}

	sunspecIDs   = []int{1, 2, 3, 71, 126} // modbus ids
	chargeStatus = []int{65, 66, 67}       // status values A..C
)

func init() {
	taskList.Add(detect.Task{
		ID:   "ping",
		Type: "ping",
	})

	taskList.Add(detect.Task{
		ID:      "tcp_502",
		Type:    "tcp",
		Depends: "ping",
		Config: map[string]interface{}{
			"port": 502,
		},
	})

	taskList.Add(detect.Task{
		ID:      "sunspec",
		Type:    "modbus",
		Depends: "tcp_502",
		Config: map[string]interface{}{
			"ids":     sunspecIDs,
			"timeout": time.Second,
		},
	})

	taskList.Add(detect.Task{
		ID:      "modbus_inverter",
		Type:    "modbus",
		Depends: "sunspec",
		Config: map[string]interface{}{
			// "port": 1502,
			"ids":     sunspecIDs,
			"models":  []int{101, 103},
			"point":   "W", // status
			"invalid": []int{65535},
			"timeout": time.Second,
		},
	})

	taskList.Add(detect.Task{
		ID:      "modbus_battery",
		Type:    "modbus",
		Depends: "sunspec",
		Config: map[string]interface{}{
			// "port": 1502,
			"ids":     sunspecIDs,
			"models":  []int{124},
			"point":   "ChaSt", // status
			"invalid": []int{65535},
			"timeout": time.Second,
		},
	})

	taskList.Add(detect.Task{
		ID:      "modbus_meter",
		Type:    "modbus",
		Depends: "sunspec",
		Config: map[string]interface{}{
			"ids":     sunspecIDs,
			"models":  []int{201, 203},
			"point":   "W",
			"timeout": time.Second,
		},
	})

	taskList.Add(detect.Task{
		ID:      "modbus_e3dc_simple",
		Type:    "modbus",
		Depends: "tcp_502",
		Config: map[string]interface{}{
			"ids":     []int{1, 2, 3, 4, 5, 6},
			"address": 40000,
			"type":    "holding",
			"decode":  "uint16",
			"values":  []int{58332}, // 0xE3DC
		},
	})

	taskList.Add(detect.Task{
		ID:      "modbus_wallbe",
		Type:    "modbus",
		Depends: "tcp_502",
		Config: map[string]interface{}{
			"ids":     []int{255},
			"address": 100,
			"type":    "input",
			"decode":  "uint16",
			"values":  chargeStatus,
		},
	})

	taskList.Add(detect.Task{
		ID:      "modbus_emcp",
		Type:    "modbus",
		Depends: "tcp_502",
		Config: map[string]interface{}{
			"ids":     []int{180},
			"address": 100,
			"type":    "input",
			"decode":  "uint16",
			"values":  chargeStatus,
		},
	})

	taskList.Add(detect.Task{
		ID:   "mqtt",
		Type: "mqtt",
	})

	taskList.Add(detect.Task{
		ID:      "openwb",
		Type:    "mqtt",
		Depends: "mqtt",
		Config: map[string]interface{}{
			"topic": "openWB",
		},
	})

	taskList.Add(detect.Task{
		ID:      "tcp_80",
		Type:    "tcp",
		Depends: "ping",
		Config: map[string]interface{}{
			"port": 80,
		},
	})

	taskList.Add(detect.Task{
		ID:      "go-e",
		Type:    "http",
		Depends: "tcp_80",
		Config: map[string]interface{}{
			"path":    "/status",
			"jq":      ".car",
			"timeout": 500 * time.Millisecond,
		},
	})

	taskList.Add(detect.Task{
		ID:      "evsewifi",
		Type:    "http",
		Depends: "tcp_80",
		Config: map[string]interface{}{
			"path":    "/getParameters",
			"jq":      ".type",
			"timeout": 500 * time.Millisecond,
		},
	})

	taskList.Add(detect.Task{
		ID:   "sonnen",
		Type: "http",
		// Depends: "tcp_80",
		Config: map[string]interface{}{
			"port":    8080,
			"path":    "/api/v1/status",
			"jq":      ".GridFeedIn_W",
			"timeout": 500 * time.Millisecond,
		},
	})

	taskList.Add(detect.Task{
		ID:      "powerwall",
		Type:    "http",
		Depends: "tcp_80",
		Config: map[string]interface{}{
			"path":    "/api/meters/aggregates",
			"jq":      ".load",
			"timeout": 500 * time.Millisecond,
		},
	})

	// taskList.Add(detect.Task{
	// 	ID:      "volkszähler",
	// 	Type:    "http",
	// 	Depends: "tcp_80",
	// 	Config: map[string]interface{}{
	// 		"path":    "/middleware.php/entity.json",
	// 		"timeout": 500 * time.Millisecond,
	// 	},
	// })
}

func workers(num int, tasks <-chan net.IP) *sync.WaitGroup {
	var wg sync.WaitGroup
	for i := 0; i < num; i++ {
		wg.Add(1)
		go func() {
			work(tasks)
			wg.Done()
		}()
	}

	return &wg
}

func work(tasks <-chan net.IP) {
	for ip := range tasks {
		taskList.Test(log, ip)
	}
}

func runDetect(cmd *cobra.Command, args []string) {
	util.LogLevel("info", nil)

	tasks := make(chan net.IP)
	wg := workers(50, tasks)

	ips := semp.LocalIPs()
	if len(ips) == 0 {
		log.FATAL.Fatal("could not find ip")
	}

	log.INFO.Println("my ip:", ips[0].IP)

	if len(args) > 0 {
		ips = nil

		for _, arg := range args {
			_, ipnet, err := net.ParseCIDR(arg + "/32")
			if err != nil {
				log.FATAL.Fatal("could not parse", arg)
			}

			ips = append(ips, *ipnet)
		}
	} else {
		tasks <- net.ParseIP("127.0.0.1")
	}

	var bar *pb.ProgressBar
	segment := 255
	// count := len(ips) * segment
	// bar = pb.StartNew(count)

	for _, ipnet := range ips {
		subnet := ipnet.String()

		if bits, _ := ipnet.Mask.Size(); bits < 24 {
			log.INFO.Println("skipping large subnet:", subnet)
			if bar != nil {
				bar.Add(segment)
			}
			continue
		}

		log.INFO.Println("subnet:", subnet)

		gen, err := ipnetgen.New(subnet)
		if err != nil {
			log.FATAL.Fatal("could not create iterator")
		}

		var count int
		for ip := gen.Next(); ip != nil; ip = gen.Next() {
			// log.INFO.Println("ip:", ip)
			tasks <- ip

			if bar != nil {
				bar.Increment()
				count++
			}
		}

		if bar != nil && count < segment {
			bar.Add(segment - count)
		}
	}

	close(tasks)
	wg.Wait()
}