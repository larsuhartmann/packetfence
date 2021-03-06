package main

import (
	"context"
	"database/sql"
	"encoding/binary"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/inverse-inc/packetfence/go/database"
	"github.com/inverse-inc/packetfence/go/log"
	"github.com/inverse-inc/packetfence/go/pfconfigdriver"
)

type NodeInfo struct {
	Mac      string
	Status   string
	Category string
}

// connectDB connect to the database
func connectDB(configDatabase pfconfigdriver.PfconfigDatabase, db *sql.DB) {
	MySQLdatabase = database.ConnectFromConfig(configDatabase)
}

// initiaLease fetch the database to remove already assigned ip addresses
func initiaLease(dhcpHandler *DHCPHandler) {
	// Need to calculate the end ip because of the ip per role feature
	endip := binary.BigEndian.Uint32(dhcpHandler.start.To4()) + uint32(dhcpHandler.leaseRange) - uint32(1)
	a := make([]byte, 4)
	binary.BigEndian.PutUint32(a, endip)
	ipend := net.IPv4(a[0], a[1], a[2], a[3])

	rows, err := MySQLdatabase.Query("select ip,mac,end_time,start_time from ip4log i where inet_aton(ip) between inet_aton(?) and inet_aton(?) and (end_time = 0 OR  end_time > NOW()) and end_time in (select MAX(end_time) from ip4log where mac = i.mac)  ORDER BY mac,end_time desc", dhcpHandler.start.String(), ipend.String())
	if err != nil {
		log.LoggerWContext(ctx).Error(err.Error())
		return
	}
	defer rows.Close()
	var (
		ipstr      string
		mac        string
		end_time   time.Time
		start_time time.Time
	)

	for rows.Next() {
		err := rows.Scan(&ipstr, &mac, &end_time, &start_time)
		if err != nil {
			log.LoggerWContext(ctx).Error(err.Error())
			return
		}

		// Calculate the leasetime from the date in the database
		now := time.Now()
		var leaseDuration time.Duration
		if end_time.IsZero() {
			leaseDuration = dhcpHandler.leaseDuration
		} else {
			leaseDuration = end_time.Sub(now)
		}
		ip := net.ParseIP(ipstr)

		// Calculate the position for the roaring bitmap
		position := uint32(binary.BigEndian.Uint32(ip.To4())) - uint32(binary.BigEndian.Uint32(dhcpHandler.start.To4()))

		// Remove the position in the roaming bitmap
		dhcpHandler.available.Remove(position)
		// Add the mac in the cache
		dhcpHandler.hwcache.Set(mac, int(position), leaseDuration)
		GlobalIpCache.Set(ipstr, mac, leaseDuration)
		GlobalMacCache.Set(mac, ipstr, leaseDuration)
	}
}

func InterfaceScopeFromMac(MAC string) string {
	var NetWork string
	if index, found := GlobalMacCache.Get(MAC); found {
		for _, v := range DHCPConfig.intsNet {
			v := v
			for network := range v.network {
				if v.network[network].network.Contains(net.ParseIP(index.(string))) {
					NetWork = v.network[network].network.String()
					if x, found := v.network[network].dhcpHandler.hwcache.Get(MAC); found {
						v.network[network].dhcpHandler.hwcache.Replace(MAC, x.(int), 3*time.Second)
						v.network[network].dhcpHandler.available.Add(uint32(x.(int)))
						log.LoggerWContext(ctx).Info(MAC + " removed")
					}
				}
			}
		}
	}
	return NetWork
}

// Detect the vip on each interfaces
func (d *Interfaces) detectVIP(interfaces pfconfigdriver.ListenInts) {

	var keyConfCluster pfconfigdriver.NetInterface
	keyConfCluster.PfconfigNS = "config::Pf(CLUSTER)"

	for _, v := range interfaces.Element {
		keyConfCluster.PfconfigHashNS = "interface " + v
		pfconfigdriver.FetchDecodeSocket(ctx, &keyConfCluster)
		// Nothing in keyConfCluster.Ip so we are not in cluster mode
		if keyConfCluster.Ip == "" {
			VIP[v] = true
			continue
		}

		if _, found := VIP[v]; !found {
			VIP[v] = false
		}

		eth, _ := net.InterfaceByName(v)
		adresses, _ := eth.Addrs()
		var found bool
		found = false
		for _, adresse := range adresses {
			IP, _, _ := net.ParseCIDR(adresse.String())
			VIPIp[v] = net.ParseIP(keyConfCluster.Ip)
			if IP.Equal(VIPIp[v]) {
				found = true
				if VIP[v] == false {
					log.LoggerWContext(ctx).Info(v + " got the VIP")
					if _, ok := ControlIn[v]; ok {
						Request := ApiReq{Req: "initialease", NetInterface: v, NetWork: ""}
						ControlIn[v] <- Request
					}
					VIP[v] = true
				}
			}
		}
		if found == false {
			VIP[v] = false
		}
	}
}

func NodeInformation(target net.HardwareAddr, ctx context.Context) (r NodeInfo) {

	rows, err := MySQLdatabase.Query("SELECT mac, status, IF(ISNULL(nc.name), '', nc.name) as category FROM node LEFT JOIN node_category as nc on node.category_id = nc.category_id WHERE mac = ?", target.String())
	defer rows.Close()

	if err != nil {
		log.LoggerWContext(ctx).Crit(err.Error())
	}

	var (
		Category string
		Status   string
		Mac      string
	)
	// Set default values
	var Node = NodeInfo{Mac: target.String(), Status: "unreg", Category: "default"}

	for rows.Next() {
		err := rows.Scan(&Mac, &Status, &Category)
		if err != nil {
			log.LoggerWContext(ctx).Crit(err.Error())

		}
	}

	Node = NodeInfo{Mac: Mac, Status: Status, Category: Category}
	return Node
}

func ShuffleDNS(ConfNet pfconfigdriver.RessourseNetworkConf) (r []byte) {
	if ConfNet.ClusterIPs != "" {
		return Shuffle(ConfNet.ClusterIPs)
	}
	if ConfNet.Dnsvip != "" {
		return []byte(net.ParseIP(ConfNet.Dnsvip).To4())
	} else {
		return []byte(net.ParseIP(ConfNet.Dns).To4())
	}
}

func ShuffleGateway(ConfNet pfconfigdriver.RessourseNetworkConf) (r []byte) {
	if ConfNet.NextHop != "" {
		return []byte(net.ParseIP(ConfNet.NextHop).To4())
	} else if ConfNet.ClusterIPs != "" {
		return Shuffle(ConfNet.ClusterIPs)
	} else {
		return []byte(net.ParseIP(ConfNet.Gateway).To4())
	}
}

func Shuffle(addresses string) (r []byte) {
	var array []net.IP
	for _, adresse := range strings.Split(addresses, ",") {
		array = append(array, net.ParseIP(adresse).To4())
	}

	slice := make([]byte, 0, len(array))

	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := len(array) - 1; i > 0; i-- {
		j := random.Intn(i + 1)
		array[i], array[j] = array[j], array[i]
	}
	for _, element := range array {
		elem := []byte(element)
		slice = append(slice, elem...)
	}
	return slice
}

func ShuffleNetIP(array []net.IP) (r []byte) {

	slice := make([]byte, 0, len(array))

	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := len(array) - 1; i > 0; i-- {
		j := random.Intn(i + 1)
		array[i], array[j] = array[j], array[i]
	}
	for _, element := range array {
		elem := []byte(element)
		slice = append(slice, elem...)
	}
	return slice
}

func ShuffleIP(a []byte) (r []byte) {

	var array []net.IP
	for len(a) != 0 {
		array = append(array, net.IPv4(a[0], a[1], a[2], a[3]).To4())
		_, a = a[0], a[4:]
	}
	return ShuffleNetIP(array)
}

// readWebservicesConfig read pfconfig webservices configuration
func readWebservicesConfig() pfconfigdriver.PfConfWebservices {
	var webservices pfconfigdriver.PfConfWebservices
	webservices.PfconfigNS = "config::Pf"
	webservices.PfconfigMethod = "hash_element"
	webservices.PfconfigHashNS = "webservices"

	pfconfigdriver.FetchDecodeSocket(ctx, &webservices)
	return webservices
}
