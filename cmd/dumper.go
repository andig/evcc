package cmd

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/andig/evcc/api"
)

var truefalse = map[bool]string{false: "false", true: "true"}

type dumper struct {
	len int
}

func (d *dumper) Header(name, underline string) {
	fmt.Println(name)
	fmt.Println(strings.Repeat(underline, len(name)))
}

func (d *dumper) DumpWithHeader(name string, device interface{}) {
	if d.len > 1 {
		d.Header(name, "-")
	}

	d.Dump(name, device)

	if d.len > 1 {
		fmt.Println()
	}
}

func (d *dumper) Dump(name string, v interface{}) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)

	// meter

	if v, ok := v.(api.Meter); ok {
		if power, err := v.CurrentPower(); err != nil {
			fmt.Fprintf(w, "Power:\t%v\n", err)
		} else {
			fmt.Fprintf(w, "Power:\t%.0fW\n", power)
		}

		if v, ok := api.GetMeterEnergy(v); ok {
			if energy, err := v.TotalEnergy(); err != nil {
				fmt.Fprintf(w, "Energy:\t%v\n", err)
			} else {
				fmt.Fprintf(w, "Energy:\t%.1fkWh\n", energy)
			}
		}

		if v, ok := api.GetMeterCurrent(v); ok {
			if i1, i2, i3, err := v.Currents(); err != nil {
				fmt.Fprintf(w, "Current L1..L3:\t%v\n", err)
			} else {
				fmt.Fprintf(w, "Current L1..L3:\t%.3gA %.3gA %.3gA\n", i1, i2, i3)
			}
		}
	}

	if v, ok := api.GetBattery(v); ok {
		soc, err := v.SoC()

		for err != nil && errors.Is(err, api.ErrMustRetry) {
			fmt.Fprint(w, ".")
			time.Sleep(3 * time.Second)
			soc, err = v.SoC()
		}

		if err != nil {
			fmt.Fprintf(w, "SoC:\t%v\n", err)
		} else {
			fmt.Fprintf(w, "SoC:\t%.0f%%\n", soc)
		}
	}

	// charger

	if v, ok := v.(api.ChargeState); ok {
		if status, err := v.Status(); err != nil {
			fmt.Fprintf(w, "Charge status:\t%v\n", err)
		} else {
			fmt.Fprintf(w, "Charge status:\t%v\n", status)
		}
	}

	if v, ok := v.(api.Charger); ok {
		if enabled, err := v.Enabled(); err != nil {
			fmt.Fprintf(w, "Enabled:\t%v\n", err)
		} else {
			fmt.Fprintf(w, "Enabled:\t%s\n", truefalse[enabled])
		}
	}

	if v, ok := v.(api.ChargeRater); ok {
		if energy, err := v.ChargedEnergy(); err != nil {
			fmt.Fprintf(w, "Charged:\t%v\n", err)
		} else {
			fmt.Fprintf(w, "Charged:\t%.1fkWh\n", energy)
		}
	}

	if v, ok := v.(api.ChargeTimer); ok {
		if duration, err := v.ChargingTime(); err != nil {
			fmt.Fprintf(w, "Duration:\t%v\n", err)
		} else {
			fmt.Fprintf(w, "Duration:\t%v\n", duration.Truncate(time.Second))
		}
	}

	// vehicle

	if v, ok := v.(api.Vehicle); ok {
		fmt.Fprintf(w, "Capacity:\t%dkWh\n", v.Capacity())
	}

	if v, ok := v.(api.VehicleRange); ok {
		if rng, err := v.Range(); err != nil {
			fmt.Fprintf(w, "Range:\t%v\n", err)
		} else {
			fmt.Fprintf(w, "Range:\t%vkm\n", rng)
		}
	}

	if v, ok := v.(api.VehicleFinishTimer); ok {
		if ft, err := v.FinishTime(); err != nil {
			fmt.Fprintf(w, "Finish time:\t%v\n", err)
		} else {
			fmt.Fprintf(w, "Finish time:\t%v\n", ft.Truncate(time.Minute))
		}
	}

	if v, ok := v.(api.VehicleClimater); ok {
		if active, ot, tt, err := v.Climater(); err != nil {
			fmt.Fprintf(w, "Climater:\t%v\n", err)
		} else {
			fmt.Fprintf(w, "Climate active:\t%v\n", active)
			if !math.IsNaN(ot) {
				fmt.Fprintf(w, "Outside temp:\t%.1f°C\n", ot)
			}
			if !math.IsNaN(tt) {
				fmt.Fprintf(w, "Target temp:\t%.1f°C\n", tt)
			}
		}
	}

	// Identity

	if v, ok := v.(api.Identifier); ok {
		if id, err := v.Identify(); err != nil {
			fmt.Fprintf(w, "Identifier:\t%v\n", err)
		} else if id != "" {
			fmt.Fprintf(w, "Identifier:\t%s\n", id)
		}
	}

	if v, ok := v.(api.Diagnosis); ok {
		fmt.Fprintln(w, "Diagnostic dump:")
		v.Diagnose()
	}

	w.Flush()
}
