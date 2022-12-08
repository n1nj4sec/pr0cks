package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
    "flag"
    prockslib "github.com/n1nj4sec/pr0cks/lib"
    dns "github.com/n1nj4sec/pr0cks/dns"
)

const (
    VersionInfo = "pr0cks v1.0 -- Nicolas Verdier (contact@n1nj4.eu)"
)


func autoconf(ldns string, lproxy string, socksserver string, dnsserver string) {

    cmd_list:=[][]string{
        []string{"-t", "nat", },
        []string{"-L"},
        }
    log.Println(cmd_list)
    for _,cmdslice := range cmd_list {
        log.Println(cmdslice)
        cmd := exec.Command("iptables", cmdslice...)
        stdout, err := cmd.Output()
        if err != nil {
            log.Println("Error running command ", err)
        }
        fmt.Printf("%s\n", stdout)
    }
}

func main() {
	// create a socks5 dialer
    var verbose bool = false
    var version bool = false
    var help bool = false
    var dnsserver string = "8.8.8.8:53"
    var socksserver string = "127.0.0.1:1080"
    var ldns string = "127.0.0.1:10053"
    var lproxy string = "127.0.0.1:10080"
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s\n", os.Args[0])
		flag.PrintDefaults()
	}
    flag.BoolVar(&verbose, "v", false, "verbose")
    flag.BoolVar(&help, "h", false, "help")
    flag.BoolVar(&version, "version", false, "print version")
    flag.StringVar(&dnsserver, "dns", dnsserver, "dns server and port")
    flag.StringVar(&socksserver, "socks5", socksserver, "socks server and port")
    flag.StringVar(&ldns, "ldns", ldns, "local dns server and port server and port")
    flag.StringVar(&lproxy, "lproxy", lproxy, "local TCP proxy server and port")
    flag.Parse()
    if(version) {
        fmt.Println(VersionInfo)
        os.Exit(0)
    }
    if(help) {
        flag.Usage()
        os.Exit(0)
    }
    if(verbose){
        fmt.Println("verbose option activated !")
    }
    //autoconf(ldns, lproxy, socksserver, dnsserver)
    //var ProxyAddress = flag.Arg(0)
	var pdns = dns.NewServer(ldns, socksserver, dnsserver)
	go pdns.Start()

    var psrv = prockslib.NewServer(lproxy, socksserver)

    psrv.Start()
}

