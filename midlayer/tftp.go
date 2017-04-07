package midlayer

import (
	"bytes"
	"io"
	"log"
	"net"
	"os"

	"github.com/digitalrebar/provision/backend"
	"github.com/pin/tftp"
)

func ServeTftp(listen string, responder func(string, net.IP) (io.Reader, error), logger *log.Logger) error {
	a, err := net.ResolveUDPAddr("udp", listen)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", a)
	if err != nil {
		return err
	}
	svr := tftp.NewServer(func(filename string, rf io.ReaderFrom) error {
		var local net.IP
		var remote net.UDPAddr
		t, outgoing := rf.(tftp.OutgoingTransfer)
		rpi, haveRPI := rf.(tftp.RequestPacketInfo)
		if outgoing && haveRPI {
			local = rpi.LocalIP()
		}
		if outgoing {
			remote = t.RemoteAddr()
		}
		if outgoing && haveRPI {
			backend.AddToCache(local, remote.IP)
		}
		source, err := responder(filename, remote.IP)
		if err != nil {
			return err
		}
		if outgoing {
			var size int64
			switch src := source.(type) {
			case *os.File:
				defer src.Close()
				if fi, err := src.Stat(); err == nil {
					size = fi.Size()
				}
			case *bytes.Reader:
				size = src.Size()
			}
			t.SetSize(size)
		}
		_, err = rf.ReadFrom(source)
		if err != nil {
			logger.Println(err)
			return err
		}
		return nil
	}, nil)

	go svr.Serve(conn)
	return nil
}
