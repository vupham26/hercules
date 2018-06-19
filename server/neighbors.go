package server

import (
	"errors"
	"net"
	"strings"

	"../logs"
)

func AddNeighbor(address string) error {
	if strings.HasPrefix(address, "tcp://") {
		return errors.New("TCP protocol is not supported yet. '" + address + "' could not be removed")
	}

	address = strings.Replace(address, "udp://", "", -1)
	hostname := ""
	identifier, port := getAddressAndPort(address)

	addr := net.ParseIP(identifier)
	if addr == nil {
		// Probably hostname. Check it
		addresses, _ := net.LookupHost(identifier)
		if len(addresses) > 0 {
			hostname = identifier
			address = addresses[0] + ":" + port
		} else {
			return errors.New("Couldn't lookup host: " + address)
		}
	}

	NeighborsLock.Lock()
	defer NeighborsLock.Unlock()

	for _, neighbor := range Neighbors {
		if neighbor.Addr == address || (len(hostname) > 0 && neighbor.Hostname == hostname) {
			return errors.New("Neighbor '" + address + "' already exists")
		}
	}

	if len(hostname) > 0 {
		identifier = hostname
	}
	Neighbors[identifier] = createNeighbor(address, hostname)
	logs.Log.Debugf("Adding neighbor '%v' with address/port '%v' and hostname '%v'",
		identifier, Neighbors[identifier].Addr, Neighbors[identifier].Hostname)
	return nil
}

func RemoveNeighbor(address string) error {
	if strings.HasPrefix(address, "tcp://") {
		return errors.New("TCP protocol is not supported yet. '" + address + "' could not be removed")
	}

	address = strings.Replace(address, "udp://", "", -1)
	tokens := strings.Split(address, ":")
	lastIndex := len(tokens) - 1
	identifier := strings.Join(tokens[:lastIndex], ":")

	NeighborsLock.Lock()
	defer NeighborsLock.Unlock()

	identifier, neighbor := getNeighborByAddress(identifier)
	if neighbor != nil {
		delete(Neighbors, identifier)
		return nil
	} else {
		return errors.New("Could not create neighbor for '" + address + "'")
	}
}

func TrackNeighbor(msg *NeighborTrackingMessage) {
	NeighborsLock.Lock()
	defer NeighborsLock.Unlock()

	_, neighbor := getNeighborByAddress(msg.Addr)

	if neighbor != nil {
		neighbor.Incoming += msg.Incoming
		neighbor.New += msg.New
		neighbor.Invalid += msg.Invalid
	}
}

func GetNeighborByAddress(address string) (string, *Neighbor) {
	NeighborsLock.Lock()
	defer NeighborsLock.Unlock()

	return getNeighborByAddress(address)
}

func UpdateHostnameAddresses() {
	NeighborsLock.Lock()
	defer NeighborsLock.Unlock()
	for identifier, neighbor := range Neighbors {
		if len(neighbor.Hostname) > 0 {
			logs.Log.Debugf("Checking %v with current address: %v", identifier, neighbor.Addr)
			_, port := getAddressAndPort(neighbor.Addr)
			addresses, _ := net.LookupHost(neighbor.Hostname)
			if len(addresses) > 0 {
				neighbor.Addr = addresses[0] + ":" + port
				logs.Log.Debugf("Refreshed Hostname address for %v: %v", neighbor.Hostname, neighbor.Addr)
				neighbor.UDPAddr, _ = net.ResolveUDPAddr("udp", neighbor.Addr)
			}
		}
	}
}

func getNeighborByAddress(address string) (string, *Neighbor) {
	identifier, _ := getAddressAndPort(address)
	for id, neighbor := range Neighbors {
		if neighbor.Addr == address || neighbor.Hostname == identifier {
			return id, neighbor
		}
	}
	return "", nil
}

func createNeighbor(address string, hostname string) *Neighbor {
	UDPAddr, _ := net.ResolveUDPAddr("udp", address)
	neighbor := Neighbor{
		Addr:           address,
		Hostname:       hostname,
		UDPAddr:        UDPAddr,
		ConnectionType: "udp", // Only UDP is currently supported
		Incoming:       0,
		New:            0,
		Invalid:        0,
	}
	return &neighbor
}

func listenNeighborTracker() {
	for msg := range NeighborTrackingQueue {
		TrackNeighbor(msg)
	}
}

func getAddressAndPort(address string) (addr string, port string) {
	tokens := strings.Split(address, ":")
	lastIndex := len(tokens) - 1
	if lastIndex > 0 {
		port = tokens[lastIndex]
	}
	addr = strings.Join(tokens[:lastIndex], ":")
	return addr, port
}
