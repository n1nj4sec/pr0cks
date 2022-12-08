package prockslib
import (
    "log"
	"net"
	"io"
    "golang.org/x/net/proxy"
    "errors"
    "unsafe"
    "syscall"
    "strconv"
    )

const(
    BuffSize= 65535
)

type Server struct {
    BindAddress string
    ProxyAddress string
	isRunning bool
}

func NewServer(BindAddress string, ProxyAddress string) *Server {
	s:=Server{BindAddress:BindAddress, ProxyAddress:ProxyAddress, isRunning:false}
	return &s
}

func (srv *Server) Start(){
    log.Println("Starting Pr0cks server on", srv.BindAddress)
	log.Printf("Using proxy socks5://%s\n",srv.ProxyAddress)
    sock, err := net.Listen("tcp", srv.BindAddress)
    if err != nil {
        log.Panicln("Error listening:", err.Error())
    }
	srv.isRunning=true
    defer sock.Close()

    log.Println("Listening on " + srv.BindAddress)
	dialer, err := proxy.SOCKS5("tcp", srv.ProxyAddress, nil, proxy.Direct)
	if err != nil {
		log.Panicln("can't connect to the proxy:", err)
	}

    for srv.isRunning {
        conn, err := sock.Accept()
        if err != nil {
            log.Println("Error accepting: ", err.Error())
        } else {
			go handleRequest(conn, dialer)
		}
    }
}

func (srv *Server) Stop(){
	srv.isRunning=false
}


func chanFromConn(conn net.Conn) chan []byte {
    c := make(chan []byte)
    go func() {
        b := make([]byte, BuffSize)
        for {
            n, err := conn.Read(b)
            if n > 0 {
                res := make([]byte, n)
                copy(res, b[:n]) // we NEED to copy the channel here to avoid race conditions
                c <- res
            }
            if err != nil {
				if err != io.EOF {
					//log.Println("Error in chanFromConn:", err)
                    close(c)
                    break
				}
                c <- nil
                close(c)
                break
            }
        }
    }()
    return c
}
func PipeSockets(conn1 net.Conn, conn2 net.Conn) error {
    chan1 := chanFromConn(conn1)
    chan2 := chanFromConn(conn2)

    for {
        select {
            case b1 := <-chan1:
                if b1 == nil{
                    return nil
                } else {
                    conn2.Write(b1)
                }
            case b2 := <-chan2:
                if b2 == nil{
                    return nil
                } else {
                    conn1.Write(b2)
                }
        }
    }
}

func handleRequest(conn net.Conn, dialer proxy.Dialer) {
    defer func() {
        if r := recover(); r != nil {
            log.Println("Panic Error in handleRequest ! :", r)
        }
    }()

	originalAddress, err:=realServerAddress(&conn)
    if err!= nil {
        log.Println("Error retrieving SO_ORIGINAL_DST")
    }

	defer conn.Close()
	log.Println("[INFO] Proxyfing TCP request to", originalAddress)

    conn2, err := dialer.Dial("tcp", originalAddress)
    if err != nil {
		log.Println(err.Error())
        return
    }

	defer conn2.Close()
    err = PipeSockets(conn, conn2)
    if err != nil {
        log.Println("PipeSockets error:", err)
    }
}


type sockaddr struct {
	family uint16
	data   [14]byte
}

const SO_ORIGINAL_DST = 80

// realServerAddress returns an intercepted connection's original destination.
func realServerAddress(conn *net.Conn) (string, error) {
	tcpConn, ok := (*conn).(*net.TCPConn)
	if !ok {
		return "", errors.New("not a TCPConn")
	}

	file, err := tcpConn.File()
	if err != nil {
		return "", err
	}

	// To avoid potential problems from making the socket non-blocking.
	tcpConn.Close()
	*conn, err = net.FileConn(file)
	if err != nil {
		return "", err
	}

	defer file.Close()
	fd := file.Fd()

	var addr sockaddr
	size := uint32(unsafe.Sizeof(addr))
	err = getsockopt(int(fd), syscall.SOL_IP, SO_ORIGINAL_DST, uintptr(unsafe.Pointer(&addr)), &size)
	if err != nil {
		return "", err
	}

	var ip net.IP
	switch addr.family {
	case syscall.AF_INET:
		ip = addr.data[2:6]
	default:
		return "", errors.New("unrecognized address family")
	}

	port := int(addr.data[0])<<8 + int(addr.data[1])

	return net.JoinHostPort(ip.String(), strconv.Itoa(port)), nil
}

func getsockopt(s int, level int, name int, val uintptr, vallen *uint32) (err error) {
	_, _, e1 := syscall.Syscall6(syscall.SYS_GETSOCKOPT, uintptr(s), uintptr(level), uintptr(name), uintptr(val), uintptr(unsafe.Pointer(vallen)), 0)
	if e1 != 0 {
		err = e1
	}
	return
}



