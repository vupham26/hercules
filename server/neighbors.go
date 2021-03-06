package server

import (
	"errors"
	"net"
	"strings"

	"../logs"
)

func AddNeighbor(address string) error {
	neighbor, err := createNeighbor(address)

	if err != nil {
		return err
	}

	NeighborsLock.Lock()
	defer NeighborsLock.Unlock()

	neighborExists, _ := checkNeighbourExists(neighbor)
	if neighborExists {
		return errors.New("Neighbor already exists")
	}

	Neighbors[neighbor.Addr] = neighbor

	logAddNeighbor(neighbor)

	return nil
}

func logAddNeighbor(neighbor *Neighbor) {
	addingLogMessage := "Adding neighbor '%v://%v'"
	if neighbor.Hostname != "" {
		addingLogMessage += " - IP Address: '%v'"
		logs.Log.Debugf(addingLogMessage, neighbor.ConnectionType, neighbor.Addr, neighbor.IP)
	} else {
		logs.Log.Debugf(addingLogMessage, neighbor.ConnectionType, neighbor.Addr)
	}
}

func RemoveNeighbor(address string) error {
	NeighborsLock.Lock()
	defer NeighborsLock.Unlock()

	neighborExists, neighbor := checkNeighbourExistsByAddress(address)
	if neighborExists {
		delete(Neighbors, neighbor.Addr)
		return nil
	}

	return errors.New("Neighbor not found")
}

func TrackNeighbor(msg *NeighborTrackingMessage) {
	NeighborsLock.Lock()
	defer NeighborsLock.Unlock()

	neighborExists, neighbor := checkNeighbourExistsByIPAddress(msg.IPAddressWithPort)
	if neighborExists {
		neighbor.Incoming += msg.Incoming
		neighbor.New += msg.New
		neighbor.Invalid += msg.Invalid
	}
}

func GetNeighborByAddress(address string) *Neighbor {
	NeighborsLock.Lock()
	defer NeighborsLock.Unlock()

	_, neighbor := checkNeighbourExistsByAddress(address)
	return neighbor
}

func GetNeighborByIPAddressWithPort(ipAddressWithPort string) *Neighbor {
	NeighborsLock.Lock()
	defer NeighborsLock.Unlock()

	_, neighbor := checkNeighbourExistsByIPAddress(ipAddressWithPort)
	return neighbor
}

func UpdateHostnameAddresses() {
	NeighborsLock.Lock()
	defer NeighborsLock.Unlock()

	for _, neighbor := range Neighbors {
		isRegisteredWithHostname := len(neighbor.Hostname) > 0
		if isRegisteredWithHostname {
			identifier, _ := getIdentifierAndPort(neighbor.Addr)
			ip, _, _ := getIPAndHostname(identifier)

			if neighbor.IP == ip {
				logs.Log.Debugf("IP address for '%v' is up-to-date ('%v')", neighbor.Hostname, neighbor.IP)
			} else {
				neighbor.UDPAddr, _ = net.ResolveUDPAddr("udp", GetFormattedAddress(ip, neighbor.Port))
				logs.Log.Debugf("Updated IP address for '%v' from '%v' to '%v'", neighbor.Hostname, ip, neighbor.IP)
				neighbor.IP = ip
			}
		}
	}
}

func createNeighbor(address string) (*Neighbor, error) {
	connectionType, identifier, port, err := getConnectionTypeAndIdentifierAndPort(address)
	if err != nil {
		return nil, err
	}

	if connectionType != UDP {
		return nil, errors.New("This protocol is not supported yet")
	}

	ip, hostname, err := getIPAndHostname(identifier)
	if err != nil {
		return nil, err
	}

	if len(strings.Split(ip, ":")) > 1 {
		ip = "[" + ip + "]"
	}

	neighbor := Neighbor{
		Hostname:       hostname,
		IP:             ip,
		Addr:           GetFormattedAddress(identifier, port),
		Port:           port,
		ConnectionType: connectionType,
		Incoming:       0,
		New:            0,
		Invalid:        0,
	}

	if connectionType == UDP {
		conType := connectionType
		neighbor.UDPAddr, err = net.ResolveUDPAddr(conType, GetFormattedAddress(neighbor.IP, port))
		if err != nil {
			return nil, err
		}
	}

	return &neighbor, nil
}

func listenNeighborTracker() {
	for msg := range NeighborTrackingQueue {
		TrackNeighbor(msg)
	}
}

func getConnectionTypeAndIdentifierAndPort(address string) (connectionType string, identifier string, port string, e error) {
	addressWithoutConnectionType, connectionType := getConnectionType(address)
	identifier, port = getIdentifierAndPort(addressWithoutConnectionType)

	if connectionType == "" || identifier == "" || port == "" {
		return "", "", "", errors.New("Address could not be loaded")
	}

	return
}

func getConnectionType(address string) (addressWithoutConnectionType string, connectionType string) {
	tokens := strings.Split(address, "://")
	addressAndPortIndex := len(tokens) - 1
	if addressAndPortIndex > 0 {
		connectionType = tokens[0]
		addressWithoutConnectionType = tokens[addressAndPortIndex]
	} else {
		connectionType = UDP // default if none is provided
		addressWithoutConnectionType = address
	}
	return
}

func getIdentifierAndPort(address string) (identifier string, port string) {
	tokens := strings.Split(address, ":")
	portIndex := len(tokens) - 1
	if portIndex > 0 {
		identifier = strings.Join(tokens[:portIndex], ":")
		port = tokens[portIndex]
	} else {
		identifier = address
		port = config.GetString("node.port") // Tries to use same port as this node
	}

	return identifier, port
}

func getIPAndHostname(identifier string) (ip string, hostname string, err error) {

	addr := net.ParseIP(identifier)
	isIPFormat := addr != nil
	if isIPFormat {
		return addr.String(), "", nil // leave hostname empty when its in IP format
	}

	// Probably domain name. Check it
	addresses, err := net.LookupHost(identifier)
	if err != nil {
		return "", "", errors.New("Could not process look up for " + identifier)
	}
	addressFound := len(addresses) > 0
	if addressFound {
		return addresses[0], identifier, nil
	}

	return "", "", errors.New("Could not resolve a hostname for " + identifier)
}

func checkNeighbourExistsByAddress(address string) (neighborExists bool, neighbor *Neighbor) {
	_, identifier, port, _ := getConnectionTypeAndIdentifierAndPort(address)
	formattedAddress := GetFormattedAddress(identifier, port)
	neighbor, neighborExists = Neighbors[formattedAddress]
	return
}

func checkNeighbourExistsByIPAddress(ipAddressWithPort string) (neighborExists bool, neighbor *Neighbor) {
	identifier, port := getIdentifierAndPort(ipAddressWithPort)
	for _, candidateNeighbor := range Neighbors {
		if candidateNeighbor.IP == identifier && candidateNeighbor.Port == port {
			return true, candidateNeighbor
		}
	}
	return
}

func checkNeighbourExists(candidateNeighbor *Neighbor) (bool, *Neighbor) {
	neighbor, neighborExists := Neighbors[candidateNeighbor.Addr]
	return neighborExists, neighbor
}

func GetFormattedAddress(identifier string, port string) string {
	return identifier + ":" + port
}
