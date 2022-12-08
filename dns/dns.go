package dns

import (
    "fmt"
    "log"
    "net"
    "io"
    "encoding/binary"
    "golang.org/x/net/proxy"
    "github.com/google/gopacket"
    "github.com/google/gopacket/layers"
    "hash/fnv"
    "sync"
)

type Cache struct {
    sync.RWMutex
    table map[uint64]interface{}
    indexes []uint64
    maxValues int
}

type Server struct {
    BindAddress string
    ProxyAddress string
    DNSServerAddress string
	isRunning bool
    DnsCache *Cache
}

/*
type CacheEntry interface {
    TTL int
    Name int
    IP net.IP
    Type

}*/

func NewDnsCache(maxValues int) *Cache{
    c:=Cache{maxValues : maxValues}
    c.table=make(map[uint64]interface{}, maxValues)
    return &c
}

func (dc *Cache) Get(b []byte) (interface{}, bool){
    h := fnv.New64a()
    h.Write(b)
    i := h.Sum64()
    dc.RLock()
    val, prs := dc.table[i]
    dc.RUnlock()
    if !prs {
        return nil, prs
    }
    return val, prs
}

func (dc *Cache) Put(index []byte, iface interface{}){
    h := fnv.New64a()
    h.Write(index)
    i := h.Sum64()
    dc.RLock()
    if _, exist := dc.table[i]; exist { //ignore
        dc.RUnlock()
        return
    }
    dc.RUnlock()

    dc.Lock()
    dc.table[i]=iface
    dc.indexes=append(dc.indexes, i)
    if len(dc.indexes) > dc.maxValues {
        log.Println("cache too big, cleaning up !")
        for _, k := range dc.indexes[dc.maxValues:] {
            delete(dc.table, k)
            //log.Println("removing", k)
        }
        dc.indexes=dc.indexes[:dc.maxValues]
        //log.Println("table is now", dc.table)
        //log.Println("indexes are now", dc.indexes)
    }
    dc.Unlock()
}


func NewServer(BindAddress string, ProxyAddress string, DNSServerAddress string) *Server {
    dc:=NewDnsCache(100)
	s:=Server{BindAddress:BindAddress, ProxyAddress:ProxyAddress, DNSServerAddress:DNSServerAddress, isRunning:false, DnsCache : dc}
	return &s
}


func GetDNS(buf []byte) *layers.DNS{

        packet := gopacket.NewPacket(buf, layers.LayerTypeDNS, gopacket.Default)
        dnsLayer := packet.Layer(layers.LayerTypeDNS)
        if dnsLayer == nil {
            log.Println("No DNS layer in udp port 53 ??")
            return nil
        }
        dnspacket:=dnsLayer.(*layers.DNS)
        /*
        fmt.Println("ID:",dnspacket.ID)
        fmt.Println("Question nb:",dnspacket.QDCount)
        fmt.Println("Answers nb:",dnspacket.ANCount)
        fmt.Println("Questions:",dnspacket.Questions)
        fmt.Println("Answers:",dnspacket.Answers)
        */
        return dnspacket
}

func DisplayAnswers(dnsr []layers.DNSResourceRecord) string{
    res:=""
    for _,a := range dnsr {
        res+=string(a.Name)+" "+a.Type.String()+" "+string(a.IP.String())+", "
    }
    return res
}


func (srv *Server) Start() {
    ServerAddr,err := net.ResolveUDPAddr("udp",srv.BindAddress)
    if err!= nil {
		log.Panicln("Error: ", err)
    }
    ServerConn, err := net.ListenUDP("udp", ServerAddr)
    if err != nil {
        log.Panicln("Error listening on", srv.BindAddress, ":" , err.Error())
    }
    defer ServerConn.Close()
	log.Printf("Starting DNS UDP/TCP translation server on %s forwarding through socks5://%s to %s\n",srv.BindAddress, srv.ProxyAddress, srv.DNSServerAddress)

	dialer, err := proxy.SOCKS5("tcp", srv.ProxyAddress, nil, proxy.Direct)

	if err != nil {
		log.Panicln("can't connect to the proxy:", err)
	}

    buf := make([]byte, 4096*4)

    for {
        n,addr,err := ServerConn.ReadFromUDP(buf)
        if err != nil {
            fmt.Println("Error in prockslib.dns.Server ",err)
        }
        dnsp:=GetDNS(buf[:n])
        if dnsp.QDCount>0 {
            question:=dnsp.Questions[0]
            if len(dnsp.Questions)>1 {
                log.Println("multiple questions : ", len(dnsp.Questions))
            }
            index:=string(question.Name)+"_"+string(question.Type.String())
            if resp,exist := srv.DnsCache.Get([]byte(index)); exist { //if the DNS request exists in cache, we answer
                resp:=resp.(*layers.DNS)
                resp.ID=dnsp.ID
                newbuf := gopacket.NewSerializeBuffer()
                opts := gopacket.SerializeOptions{}
                //we remove Unknown type additional records, because it makes gopacket SerializeTo crash
                if len(resp.Additionals)>0 {
                    new_list:=make([]layers.DNSResourceRecord, 0)
                    for _, a := range resp.Additionals {
                        if a.Type.String()!="Unknown" {
                            new_list=append(new_list,a)
                        }
                    }
                    resp.Additionals=new_list
                    resp.ARCount=uint16(len(resp.Additionals))
                }
                err := resp.SerializeTo(newbuf, opts)
                if err!= nil {
                    log.Println("Can't serialize DNS data !", err)
                    continue
                }
                log.Println("DNS response Served from cache :> ", string(question.Name), question.Type.String(), ";>", DisplayAnswers(resp.Answers))
                _, err = ServerConn.WriteToUDP(newbuf.Bytes(), addr)
                if err != nil {
                    log.Panicln("error writing Cache UDP response:", err)
                }
            } else {
                handleUDPConnection(srv, dialer, srv.DNSServerAddress, ServerConn, addr, buf, n, string(question.Name), string(question.Type.String()))
            }
        } else {
            log.Println("No question in DNS pkt ?")
        }
    }
}

func handleUDPConnection(srv *Server, dialer proxy.Dialer, DNSServerAddress string, conn *net.UDPConn, addr *net.UDPAddr, buf []byte, size int, qName string, qType string){
    defer func() {
        if r := recover(); r != nil {
            log.Println("Panic Error in prockslib.dns.handleUDPConnection ! :", r)
        }
    }()

    log.Println("DNS request forwarded from", addr, "to", DNSServerAddress)
    //fmt.Println("Received from UDP client :  ", buf[:size])
    //fmt.Println("Received ", buf[0:size], " from ", addr)



    conn2, err := dialer.Dial("tcp", DNSServerAddress)
    if err != nil {
		log.Panicln("error dialing:", err)
    }
	defer conn2.Close()


    binsize:=make([]byte, 2)
    binary.BigEndian.PutUint16(binsize, uint16(size))
    _, err = conn2.Write(append(binsize, buf[0:size]...))

    if err != nil {
		log.Panicln("Error: ", err)
    }
    //response, err := ioutil.ReadAll(conn2)
    var size2read int16
    err = binary.Read(conn2, binary.BigEndian, &size2read)
    if err != nil {
        if err == io.EOF {
            return
        }
		log.Panicln("error reading the TCP socket response size", err)
    }
    response := make([]byte, int(size2read))
    _, err = io.ReadFull(conn2, response)
    if err != nil {
		log.Panicln("error reading the TCP socket response ", err)
    }
    dnsp:=GetDNS(response)
    if dnsp != nil {
        srv.DnsCache.Put([]byte(qName+"_"+qType), dnsp)

        log.Println("DNS response Served :>", string(qName), qType, DisplayAnswers(dnsp.Answers))

        _, err = conn.WriteToUDP(response, addr)
        if err != nil {
            log.Panicln("error writing UDP response:", err)
        }
    }
}

