package main

import (
	"net"
	"fmt"
	"os"
	"encoding/binary"
	"time"
	"strconv"
	"sort"
)

type A2SInfo struct {
	header uint8
	protocol uint8
	name string
	mapname string
	folder string
	game string
	id int16
	players uint8
	max_players uint8
	bots uint8
	server_type uint8
	environment uint8
	visibility uint8
	vac uint8
	// we asume here that nobody wants to do something with The Ship
	version string
	edf uint8 //Extra Data Flag
	timestamp time.Time //Time when this was created
}

type Server struct {
	addrport string
	info A2SInfo
}

type ByIP []Server

func (a ByIP) Len() int { return len(a) }
func (a ByIP) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByIP) Less(i, j int) bool { return a[i].addrport < a[j].addrport }

func send_info(conn net.UDPConn, addr string) {
	info_msg := []byte("\xFF\xFF\xFF\xFFTSource Engine Query\x00")

	dst, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		fmt.Println("Error while resolving remote addr")
		os.Exit(1)
	}

	_, err = conn.WriteTo(info_msg, dst)
	if err != nil {
		fmt.Println("Error while sending")
		os.Exit(1)
	}
}

func recv_answer(conn net.UDPConn) ([]byte, net.UDPAddr, net.Error) {
	buf := make([]byte, 1500)

	amount, addr, err := conn.ReadFromUDP(buf)
	if err != nil {
		err, ok := err.(*net.OpError)
		if ok {
			return []byte{}, net.UDPAddr{}, err
		}

		fmt.Println("An error occured while receiving", err)
		os.Exit(1)
	}

	return buf[0:amount], *addr, nil
}

func parse_byte(buf []byte, pos *int) uint8 {
	value := uint8(buf[*pos])
	*pos += 1
	return value
}
func parse_short(buf []byte, pos *int) int16 {
	value := int16(binary.LittleEndian.Uint16(buf[*pos:*pos+2]))
	*pos += 2
	return value
}
func parse_string(buf []byte, pos *int) string {
	// TODO
	start := *pos
	for buf[*pos] != 0 {
		*pos++
	}
	value := string(buf[start:*pos])
	*pos++
	return value
}

func parseResponse(buf []byte) A2SInfo {
	var resp A2SInfo
	pos := 4
	resp.header = parse_byte(buf, &pos)
	resp.protocol = parse_byte(buf, &pos)
	resp.name = parse_string(buf, &pos)
	resp.mapname = parse_string(buf, &pos)
	resp.folder= parse_string(buf, &pos)
	resp.game = parse_string(buf, &pos)
	resp.id = parse_short(buf, &pos)
	resp.players = parse_byte(buf, &pos)
	resp.max_players = parse_byte(buf, &pos)
	resp.bots = parse_byte(buf, &pos)
	resp.server_type = parse_byte(buf, &pos)
	resp.environment = parse_byte(buf, &pos)
	resp.visibility = parse_byte(buf, &pos)
	resp.vac = parse_byte(buf, &pos)
	resp.edf = parse_byte(buf, &pos)
	resp.timestamp = time.Now()

	return resp
}



func main() {
	delay, _ := time.ParseDuration("500ms")

	hosts := []string{
		"4.5.6.7:27015",
		"87.98.228.196:27040",
		"216.52.148.47:27015",
		"74.91.113.83:27015",
		"13.73.0.133:27017",
		"46.174.54.231:27015",
		"54.37.111.216:27015",
		"185.193.165.14:27015",
		"89.40.104.120:27015",
		"188.212.100.42:27015",
		"193.33.176.95:27015",
		"74.91.119.186:27015",
		"193.33.176.106:27015",
		"74.91.113.113:27015",
		"145.239.149.122:27015",
		"217.11.249.78:27242"}

	data := make(map[string]A2SInfo)

	for _, server := range hosts {
		data[server] = A2SInfo{}
	}

	addr, err := net.ResolveUDPAddr("udp", ":0")

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Println("Error while dialing")
		os.Exit(1)
	}
	defer conn.Close()

	for {

		for server, _ := range data {
			send_info(*conn, server)
		}

		conn.SetReadDeadline(time.Now().Add(delay))
		for i := 0; i< len(data); i++ {
			buf, addr, err := recv_answer(*conn)
			if err != nil {
				if err.Timeout() {
					break
				}
			}

			addr4 := addr.IP.To4()
			key := addr4.String() + ":" + strconv.Itoa(addr.Port)


			info := parseResponse(buf)

			data[key] = info

		}
		serverarray := make([]Server, 0, len(data))
		for ip, info := range data {
			serverarray = append(serverarray, Server{ip, info})
		}

		sort.Sort(ByIP(serverarray))

		fmt.Printf("\033[2J")
		fmt.Printf("\033[0;0H")

		for _, server := range serverarray {
			addr := server.addrport
			info := server.info
			fmt.Printf("%s %2d(%2d)/%2d %s %f\n", addr, info.players, info.bots, info.max_players, info.mapname, time.Since(info.timestamp).Seconds())
		}
		fmt.Println("---------------------------------------------------")
	}
}

