package main

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/mdlayher/ndp"
)

const (
	raHopLimit      = 0
	raDelay         = 500 * time.Millisecond
	raDefaultPeriod = 3 * time.Minute
)

var (
	ipv6LinkLocalAllRouters = netip.MustParseAddr(net.IPv6linklocalallrouters.String())
	ipv6LinkLocalAllNodes   = netip.MustParseAddr(net.IPv6linklocalallnodes.String())
)

// SendPeriodicRouterAdvertisements sends periodic router advertisement messages.
func SendPeriodicRouterAdvertisements(ctx context.Context, logger *Logger, config Config) error {
	iface, err := net.InterfaceByName(config.Interface.Name)
	if err != nil {
		return fmt.Errorf("selecting interface %q: %w", config.Interface.Name, err)
	}

	conn, _, err := ndp.Listen(config.Interface, ndp.LinkLocal)
	if err != nil {
		return fmt.Errorf("dialing (%s): %w", config.Interface.Name, err)
	}

	defer func() { _ = conn.Close() }()

	err = conn.JoinGroup(ipv6LinkLocalAllRouters)
	if err != nil {
		return fmt.Errorf("joining multicast group: %w", err)
	}

	time.Sleep(raDelay) // time for DHCPv6 server to start

	for {
		logger.Infof("sending router advertisement on %s", iface.Name)

		err := sendRouterAdvertisement(conn, iface.HardwareAddr)
		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(config.RAPeriod):
			continue
		}
	}
}

func sendRouterAdvertisement(c *ndp.Conn, routerMAC net.HardwareAddr) error {
	m := &ndp.RouterAdvertisement{
		CurrentHopLimit:      raHopLimit,
		ManagedConfiguration: true,
		OtherConfiguration:   true,

		RouterSelectionPreference: ndp.High,
		RouterLifetime:            0, // RFC4861 (Section 4.2)
		Options: []ndp.Option{
			&ndp.LinkLayerAddress{
				Direction: ndp.Source,
				Addr:      routerMAC,
			},
		},
	}

	err := c.WriteTo(m, nil, ipv6LinkLocalAllNodes)
	if err != nil {
		return fmt.Errorf("sending router advertisement: %w", err)
	}

	return nil
}
