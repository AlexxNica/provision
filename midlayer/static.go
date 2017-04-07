package midlayer

import (
	"log"
	"net"
	"net/http"

	"github.com/digitalrebar/provision/backend"
)

func ServeStatic(listenAt string, responder http.Handler, logger *log.Logger) error {
	conn, err := net.Listen("tcp", listenAt)
	if err != nil {
		return err
	}
	svr := &http.Server{
		Addr:    listenAt,
		Handler: responder,
		ConnState: func(n net.Conn, cs http.ConnState) {
			laddr, lok := n.LocalAddr().(*net.TCPAddr)
			raddr, rok := n.RemoteAddr().(*net.TCPAddr)
			if lok && rok && cs == http.StateActive {
				backend.AddToCache(laddr.IP, raddr.IP)
			}
			return
		},
	}
	go func() {
		if err := svr.Serve(conn); err != nil {
			logger.Fatalf("Static HTTP server error %v", err)
		}
	}()
	return nil
}
