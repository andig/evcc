package detect

import (
	"net"
	"time"

	"github.com/andig/evcc/util"
	"github.com/go-ping/ping"
)

func init() {
	registry.Add("ping", PingHandlerFactory)
}

func PingHandlerFactory(conf map[string]interface{}) (TaskHandler, error) {
	handler := PingHandler{
		Count:   1,
		Timeout: timeout,
	}

	err := util.DecodeOther(conf, &handler)

	return &handler, err
}

type PingHandler struct {
	Count   int
	Timeout time.Duration
}

func (h *PingHandler) Test(log *util.Logger, ip net.IP) bool {
	pinger, err := ping.NewPinger(ip.String())
	if err != nil {
		panic(err)
	}

	pinger.Count = h.Count
	pinger.Timeout = h.Timeout

	err = pinger.Run()
	if err != nil {
		log.ERROR.Println("ping:", err)
	}

	stat := pinger.Statistics()
	if err != nil {
		log.ERROR.Printf("ping: %+v\n", stat)
	}

	return stat.PacketsRecv > 0
}