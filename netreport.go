package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// run executes a command and returns its combined output.
func run(cmd string, args ...string) (string, error) {
	c := exec.Command(cmd, args...)
	var out bytes.Buffer
	c.Stdout = &out
	c.Stderr = &out
	err := c.Run()
	return out.String(), err
}

// getCIDR returns the CIDR of the default network interface and the interface name.
func getCIDR() (string, string, error) {
	out, err := run("sh", "-c", "ip route | awk '/default/ {print $5}'")
	if err != nil {
		return "", "", fmt.Errorf("get default interface: %v: %s", err, out)
	}
	iface := strings.TrimSpace(out)
	if iface == "" {
		return "", "", fmt.Errorf("default interface not found")
	}
	out, err = run("sh", "-c", fmt.Sprintf("ip -o -f inet addr show dev %s | awk '{print $4}'", iface))
	if err != nil {
		return "", iface, fmt.Errorf("get CIDR: %v: %s", err, out)
	}
	cidr := strings.TrimSpace(out)
	return cidr, iface, nil
}

// getWifiInterface returns the first wireless interface found via iw.
func getWifiInterface() (string, error) {
	out, err := run("sh", "-c", "iw dev | awk '$1==\"Interface\"{print $2}'")
	if err != nil {
		return "", fmt.Errorf("list wifi interfaces: %v: %s", err, out)
	}
	lines := strings.Fields(out)
	if len(lines) == 0 {
		return "", fmt.Errorf("no wifi interface found")
	}
	return lines[0], nil
}

func main() {
	start := time.Now()

	cidr, iface, err := getCIDR()
	if err != nil {
		fmt.Printf("CIDR error: %v\n", err)
	}

	wifiIf, err := getWifiInterface()
	if err != nil {
		fmt.Printf("WiFi interface error: %v\n", err)
	}

	var wg sync.WaitGroup
	results := struct {
		nmap  string
		freq  string
		speed string
	}{}

	// Nmap scan of connected devices
	wg.Add(1)
	go func() {
		defer wg.Done()
		if cidr == "" {
			results.nmap = "CIDR not found"
			return
		}
		out, err := run("nmap", "-sn", cidr)
		if err != nil {
			results.nmap = fmt.Sprintf("nmap error: %v\n%s", err, out)
		} else {
			results.nmap = out
		}
	}()

	// Wireless link frequency
	wg.Add(1)
	go func() {
		defer wg.Done()
		if wifiIf == "" {
			results.freq = "wifi interface not found"
			return
		}
		out, err := run("iw", "dev", wifiIf, "link")
		if err != nil {
			results.freq = fmt.Sprintf("iw error: %v\n%s", err, out)
			return
		}
		// parse freq line
		freq := ""
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "freq:") {
				freq = strings.TrimSpace(line)
				break
			}
		}
		if freq == "" {
			results.freq = out
		} else {
			results.freq = freq
		}
	}()

	// Speedtest to a São Paulo server (example ID 3696)
	wg.Add(1)
	go func() {
		defer wg.Done()
		out, err := run("speedtest", "--server-id", "3696", "--simple")
		if err != nil {
			results.speed = fmt.Sprintf("speedtest error: %v\n%s", err, out)
		} else {
			results.speed = out
		}
	}()

	wg.Wait()

	fmt.Printf("Relatório de Rede - %s\n\n", start.Format("02/01/2006 15:04:05"))
	fmt.Printf("Interface padrão: %s\n", iface)
	fmt.Printf("CIDR: %s\n\n", cidr)

	fmt.Println("Dispositivos conectados (nmap -sn):")
	fmt.Println(results.nmap)

	fmt.Printf("Interface WiFi: %s\n", wifiIf)
	fmt.Printf("Frequência de conexão:\n%s\n\n", results.freq)

	fmt.Println("Speedtest (São Paulo):")
	fmt.Println(results.speed)
}
