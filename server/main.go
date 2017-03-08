// Package main Rocket Skates Server
//
// An RestFUL API-driven Provisioner and DHCP server
//
// Terms Of Service:
//
// There are no TOS at this moment, use at your own risk we take no responsibility
//
//     Schemes: https
//     BasePath: /api/v3
//     Version: 0.1.0
//     License: APL https://raw.githubusercontent.com/digitalrebar/digitalrebar/master/LICENSE.md
//     Contact: Greg Althaus<greg@rackn.com> http://rackn.com
//
//     Consumes:
//     - application/json
//
//     Produces:
//     - application/json
//
// swagger:meta
package main

//
// Yes - Twice - once to get the basic pieces in place to let swagger run, then the final parts
//
//go:generate go-bindata -prefix ../embedded -pkg embedded -o ../embedded/embed.go ../embedded/assets/...
//go:generate swagger generate spec -o ../embedded/assets/swagger.json
//go:generate go-bindata -prefix ../embedded -pkg embedded -o ../embedded/embed.go ../embedded/assets/...

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/digitalrebar/digitalrebar/go/common/client"
	"github.com/digitalrebar/digitalrebar/go/common/service"
	"github.com/digitalrebar/digitalrebar/go/common/store"
	"github.com/digitalrebar/digitalrebar/go/common/version"
	consul "github.com/hashicorp/consul/api"
	"github.com/jessevdk/go-flags"
	"github.com/rackn/rocket-skates/backend"
	"github.com/rackn/rocket-skates/frontend"
	"github.com/rackn/rocket-skates/midlayer"
)

type ProgOpts struct {
	VersionFlag bool `long:"version" description:"Print Version and exit"`

	BackEndType string `long:"backend" description:"Storage backend to use. Can be either 'consul' or 'directory'" default:"directory"`
	DataRoot    string `long:"data-root" description:"Location we should store runtime information in" default:"digitalrebar"`

	OurAddress string `long:"static-ip" description:"IP address to advertise for the static HTTP file server" default:"192.168.124.11"`
	StaticPort int    `long:"static-port" description:"Port the static HTTP file server should listen on" default:"8091"`
	TftpPort   int    `long:"tftp-port" description:"Port for the TFTP server to listen on" default:"69"`
	ApiPort    int    `long:"api-port" description:"Port for the API server to listen on" default:"8092"`

	FileRoot string `long:"file-root" description:"Root of filesystem we should manage" default:"tftpboot"`

	DisableProvisioner bool   `long:"disable-provisioner" description:"Disable provisioner"`
	DisableDHCP        bool   `long:"disable-dhcp" description:"Disable DHCP"`
	CommandURL         string `long:"endpoint" description:"DigitalRebar Endpoint" env:"EXTERNAL_REBAR_ENDPOINT"`
	DefaultBootEnv     string `long:"default-boot-env" description:"The default bootenv for the nodes"`
	UnknownBootEnv     string `long:"unknown-boot-env" description:"The unknown bootenv for the system"`

	ExcludeDiscovery bool   `long:"exclude-discovery" description:"Should NOT download discovery image"`
	SledgeHammerURL  string `long:"sledgehammer-url" description:"Sledgehammer download URL" default:"http://opencrowbar.s3-website-us-east-1.amazonaws.com/sledgehammer"`
	SledgeHammerHash string `long:"sledgehammer-hash" description:"Sledgehammer Hash Identifier" default:"a42c8c66a60b77ca1c769b8dc7e712f6644579ed"`

	TlsKeyFile  string `long:"tls-key" description:"The TLS Key File" default:"server.key"`
	TlsCertFile string `long:"tls-cert" description:"The TLS Cert File" default:"server.crt"`

	RegisterConsul bool `long:"register-consul" description:"Register services with Consul"`
}

var c_opts ProgOpts

func main() {
	var err error

	logger := log.New(os.Stderr, "provisioner-mgmt", log.LstdFlags|log.Lmicroseconds|log.LUTC)

	parser := flags.NewParser(&c_opts, flags.Default)
	if _, err = parser.Parse(); err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			os.Exit(1)
		}
	}

	if c_opts.VersionFlag {
		logger.Fatalf("Version: %s", version.REBAR_VERSION)
	}
	logger.Printf("Version: %s\n", version.REBAR_VERSION)

	var consulClient *consul.Client
	if c_opts.RegisterConsul {
		consulClient, err = client.Consul(true)
		if err != nil {
			logger.Fatalf("Error talking to Consul: %v", err)
		}

		// Register service with Consul before continuing
		if err = service.Register(consulClient,
			&consul.AgentServiceRegistration{
				Name: "provisioner-service",
				Tags: []string{"deployment:system"},
				Port: c_opts.StaticPort,
				Check: &consul.AgentServiceCheck{
					HTTP:     fmt.Sprintf("http://[::]:%d/", c_opts.StaticPort),
					Interval: "10s",
				},
			},
			true); err != nil {
			log.Fatalf("Failed to register provisioner-service with Consul: %v", err)
		}

		if err = service.Register(consulClient,
			&consul.AgentServiceRegistration{
				Name: "provisioner-mgmt-service",
				Tags: []string{"revproxy"}, // We want to be exposed through the revproxy
				Port: c_opts.ApiPort,
				Check: &consul.AgentServiceCheck{
					HTTP:     fmt.Sprintf("http://[::]:%d/", c_opts.StaticPort),
					Interval: "10s",
				},
			},
			false); err != nil {
			log.Fatalf("Failed to register provisioner-mgmt-service with Consul: %v", err)
		}
		if err = service.Register(consulClient,
			&consul.AgentServiceRegistration{
				Name: "provisioner-tftp-service",
				Port: c_opts.TftpPort,
				Check: &consul.AgentServiceCheck{
					HTTP:     fmt.Sprintf("http://[::]:%d/", c_opts.StaticPort),
					Interval: "10s",
				},
			},
			true); err != nil {
			log.Fatalf("Failed to register provisioner-tftp-service with Consul: %v", err)
		}
	}

	var backendStore store.SimpleStore
	switch c_opts.BackEndType {
	case "consul":
		if consulClient == nil {
			consulClient, err = client.Consul(true)
			if err != nil {
				logger.Fatalf("Error talking to Consul: %v", err)
			}
		}
		backendStore, err = store.NewSimpleConsulStore(consulClient, c_opts.DataRoot)
	case "directory":
		backendStore, err = store.NewFileBackend(c_opts.DataRoot)
	case "memory":
		backendStore = store.NewSimpleMemoryStore()
		err = nil
	case "bolt", "local":
		backendStore, err = store.NewSimpleLocalStore(c_opts.DataRoot)
	default:
		logger.Fatalf("Unknown storage backend type %v\n", c_opts.BackEndType)
	}
	if err != nil {
		logger.Fatalf("Error using backing store %s: %v", c_opts.BackEndType, err)
	}

	dt := backend.NewDataTracker(backendStore,
		!c_opts.DisableProvisioner,
		!c_opts.DisableDHCP,
		c_opts.FileRoot,
		c_opts.CommandURL,
		c_opts.DefaultBootEnv,
		c_opts.UnknownBootEnv,
		fmt.Sprintf("http://%s:%d/", c_opts.OurAddress, c_opts.StaticPort),
		c_opts.OurAddress,
		logger)

	// We have a backend, now get default assets
	logger.Printf("Extracting Default Assets\n")
	if err := dt.ExtractAssets(); err != nil {
		logger.Fatalf("Unable to extract assets: %v", err)
	}
	// Add discovery image pieces if not excluded
	if !c_opts.ExcludeDiscovery {
		logger.Printf("Installing Discovery Image - could take a long time\n")
		cmd := exec.Command("./install-sledgehammer.sh", c_opts.SledgeHammerHash, c_opts.SledgeHammerURL)
		cmd.Dir = c_opts.FileRoot
		err := cmd.Run()
		if err != nil {
			log.Fatal(err)
		}
	}
	// Load default templates and bootenvs

	// Load additional config dirs. ???

	fe := frontend.NewFrontend(dt, logger, c_opts.FileRoot)

	if _, err := os.Stat(c_opts.TlsCertFile); os.IsNotExist(err) {
		buildKeys(c_opts.TlsCertFile, c_opts.TlsKeyFile)
	}

	go func() {
		if err = http.ListenAndServeTLS(fmt.Sprintf(":%d", c_opts.ApiPort), c_opts.TlsCertFile, c_opts.TlsKeyFile, fe.MgmtApi); err != nil {
			log.Fatalf("Error running API service: %v", err)
		}
	}()
	if err = midlayer.StartDhcpHandlers(dt, c_opts.OurAddress); err != nil {
		log.Fatalf("Error starting DHCP server: %v", err)
	}
	if err = frontend.ServeTftp(fmt.Sprintf(":%d", c_opts.TftpPort), c_opts.FileRoot); err != nil {
		log.Fatalf("Error starting TFTP server: %v", err)
	}
	// Static file server must always be last, as all our health checks key off of it.
	if err = frontend.ServeStatic(fmt.Sprintf(":%d", c_opts.StaticPort), c_opts.FileRoot); err != nil {
		log.Fatalf("Error starting static file server: %v", err)
	}
}
