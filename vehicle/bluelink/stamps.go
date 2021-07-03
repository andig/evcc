package bluelink

import (
	"bufio"
	_ "embed" // embedded files
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/andig/evcc/util"
	"github.com/andig/evcc/util/request"
)

//go:embed 693a33fa-c117-43f2-ae3b-61a02d24f417
var kia string

//go:embed 99cfff84-f4e2-4be8-a5ed-e5b755eb6581
var hyundai string

// Stamps collects stamps for a single brand
type Stamps map[string][]string

var stamps = Stamps{
	"693a33fa-c117-43f2-ae3b-61a02d24f417": unpack(kia),
	"99cfff84-f4e2-4be8-a5ed-e5b755eb6581": unpack(hyundai),
}

var (
	mu      sync.Mutex
	updater map[string]struct{} = make(map[string]struct{})

	client = request.NewHelper(util.NewLogger("http"), request.WithMetricsPush)
	brands = map[string]string{
		"693a33fa-c117-43f2-ae3b-61a02d24f417": "kia",
		"99cfff84-f4e2-4be8-a5ed-e5b755eb6581": "hyundai",
	}
)

func download(log *util.Logger, id, brand string) {
	var res []string
	uri := fmt.Sprintf("https://raw.githubusercontent.com/neoPix/bluelinky-stamps/master/%s.json", brand)

	err := client.GetJSON(uri, &res)
	if err != nil {
		log.ERROR.Println(err)
		return
	}

	if len(res) < 100 {
		return
	}

	mu.Lock()
	stamps[id] = res
	mu.Unlock()
}

// updateStamps updates stamps according to https://github.com/Hacksore/bluelinky/pull/144
func updateStamps(log *util.Logger, id string) {
	if _, ok := updater[id]; ok {
		return
	}

	updater[id] = struct{}{}
	download(log, id, brands[id])

	go func() {
		for range time.NewTicker(24 * time.Hour).C {
			download(log, id, brands[id])
		}
	}()
}

func unpack(source string) (res []string) {
	scanner := bufio.NewScanner(strings.NewReader(source))
	for scanner.Scan() {
		res = append(res, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
	return
}

// New creates a new stamp
func (s Stamps) New(id string) string {
	mu.Lock()
	defer mu.Unlock()

	source, ok := s[id]
	if !ok {
		panic(id)
	}
	return source[rand.Intn(len(source))]
}
