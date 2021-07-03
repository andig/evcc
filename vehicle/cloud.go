package vehicle

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/andig/evcc/api"
	"github.com/andig/evcc/api/proto/pb"
	"github.com/andig/evcc/provider"
	"github.com/andig/evcc/util"
	"github.com/andig/evcc/util/cloud"
	"github.com/andig/evcc/util/request"
)

// Cloud is an api.Vehicle implementation for Cloud cars
type Cloud struct {
	*embed
	token        string
	brand        string
	config       map[string]string
	client       pb.VehicleClient
	vehicleID    int64
	chargeStateG func() (float64, error)
}

func init() {
	registry.Add("cloud", NewCloudFromConfig)
}

// NewCloudFromConfig creates a new vehicle
func NewCloudFromConfig(other map[string]interface{}) (api.Vehicle, error) {
	cc := struct {
		Token string
		embed `mapstructure:",squash"`
		Brand string
		Other map[string]string `mapstructure:",remain"`
		Cache time.Duration
	}{
		Cache: interval,
	}

	if err := util.DecodeOther(other, &cc); err != nil {
		return nil, err
	}

	if cc.Token == "" {
		return nil, errors.New("missing required token")
	}

	host := util.Getenv("GRPC_URI", cloud.Grpc)
	conn, err := cloud.Connection(host)
	if err != nil {
		return nil, err
	}

	v := &Cloud{
		embed:  &cc.embed,
		token:  cc.Token,
		brand:  cc.Brand,
		config: cc.Other,
		client: pb.NewVehicleClient(conn),
	}

	if err == nil {
		err = v.prepareVehicle()
	}

	v.chargeStateG = provider.NewCached(v.chargeState, cc.Cache).FloatGetter()

	return v, err
}

// prepareVehicle obtains new vehicle handle from cloud server
func (v *Cloud) prepareVehicle() error {
	req := &pb.NewRequest{
		Token:  v.token,
		Type:   v.brand,
		Config: v.config,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*request.Timeout)
	defer cancel()

	res, err := v.client.New(ctx, req)
	if err == nil {
		v.vehicleID = res.VehicleId
	}

	return err
}

// chargeState implements the api.Vehicle interface
func (v *Cloud) chargeState() (float64, error) {
	req := &pb.SoCRequest{
		Token:     v.token,
		VehicleId: v.vehicleID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), request.Timeout)
	defer cancel()

	res, err := v.client.SoC(ctx, req)

	if err != nil && strings.Contains(err.Error(), api.ErrMustRetry.Error()) {
		return 0, api.ErrMustRetry
	}

	if err != nil && strings.Contains(err.Error(), cloud.ErrVehicleNotAvailable.Error()) && v.prepareVehicle() == nil {
		req.VehicleId = v.vehicleID
		res, err = v.client.SoC(ctx, req)
	}

	return res.GetSoc(), err
}

// SoC implements the api.Vehicle interface
func (v *Cloud) SoC() (float64, error) {
	return v.chargeStateG()
}
