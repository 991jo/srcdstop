package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sort"
	"strings"
	"strconv"
	"time"
)

type A2SInfo struct {
	header      uint8
	protocol    uint8
	name        string
	mapname     string
	folder      string
	game        string
	id          int16
	players     uint8
	max_players uint8
	bots        uint8
	server_type uint8
	environment uint8
	visibility  uint8
	vac         uint8
	// we assume here that nobody wants to do something with The Ship
	version   string
	edf       uint8     //Extra Data Flag
	timestamp time.Time //Time when this was created
}

type Server struct {
	addrport string
	info     A2SInfo
}

type ByIP []Server

func (a ByIP) Len() int           { return len(a) }
func (a ByIP) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByIP) Less(i, j int) bool { return a[i].addrport < a[j].addrport }

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}

func getTtySize() (int, int, error) {
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}
	size := string(out)
	sizes := strings.Split(size, " ")
	x, _ := strconv.Atoi(strings.TrimSpace(sizes[1]))
	y, _ := strconv.Atoi(strings.TrimSpace(sizes[0]))
	fmt.Println(sizes[1])
	return x, y, nil
}

func sendInfo(conn net.UDPConn, addr string) {
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

func recvAnswer(conn net.UDPConn) ([]byte, net.UDPAddr, net.Error) {
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

// parses the next byte starting at pos as an uint8
func parse_byte(buf []byte, pos *int) uint8 {
	value := uint8(buf[*pos])
	*pos += 1
	return value
}

// parses the next 2 bytes starting at pos as an int16 (LittleEndian)
func parse_short(buf []byte, pos *int) int16 {
	value := int16(binary.LittleEndian.Uint16(buf[*pos : *pos+2]))
	*pos += 2
	return value
}

// parses from pos until a null byte appears (or until end of slice)
func parse_string(buf []byte, pos *int) string {
	start := *pos
	for buf[*pos] != 0 && *pos < len(buf){
		*pos++
	}
	value := string(buf[start:*pos])
	*pos++
	return value
}

func padIPPort(ipport string) string {
	for i := 0; i < len(ipport); i++ {
		if string(ipport[i]) == string(":") {
			lpad := strings.Repeat(" ", 15-i)
			rpad := strings.Repeat(" ", 21 - len(lpad) - len(ipport))
			return lpad + ipport + rpad
		}
	}
	return ""
}

func parseResponse(buf []byte) A2SInfo {
	var resp A2SInfo
	pos := 4
	resp.header = parse_byte(buf, &pos)
	resp.protocol = parse_byte(buf, &pos)
	resp.name = parse_string(buf, &pos)
	resp.mapname = parse_string(buf, &pos)
	resp.folder = parse_string(buf, &pos)
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

	hosts := os.Args[1:]

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
			sendInfo(*conn, server)
		}

		conn.SetReadDeadline(time.Now().Add(delay))
		for i := 0; i < len(data); i++ {
			buf, addr, err := recvAnswer(*conn)
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

		// reset cursor position to top left and clear the screen
		fmt.Printf("\033[2J")
		fmt.Printf("\033[0;0H")

		for _, server := range serverarray {
			addr := server.addrport
			info := server.info
			// arbitrary date that should be after the default value of a time.Date
			if info.timestamp.Before(time.Date(2000, time.January, 0,0,0,0,0, time.UTC)) {
				fmt.Printf("\033[1m%s never seen\033[0m\n", padIPPort(addr))
			} else if time.Now().Sub(info.timestamp).Seconds() > 2 {
				fmt.Printf("\033[1m%s %2d(%2d)/%2d %s\033[0m\n",
						padIPPort(addr), info.players, info.bots,
						info.max_players, info.mapname[:min(len(info.mapname),15)])
			} else {
				fmt.Printf("%s %2d(%2d)/%2d %s\n",
						padIPPort(addr), info.players, info.bots, info.max_players,
						info.mapname[:min(len(info.mapname),15)])
			}
		}
		fmt.Println("---------------------------------------------------")

		time.Sleep(time.Second)
	}
}
