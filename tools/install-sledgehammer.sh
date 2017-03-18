#!/bin/bash

discovery='---
Name: discovery
Description: "The boot environment to use to have unknown machines boot to Sledgehammer"
OS:
  Name: "sledgehammer/::SIG::"
  IsoFile: "sledgehammer-::SIG::.tar"
Kernel: vmlinuz0
Initrds: 
  - "stage1.img"
BootParams: "rootflags=loop root=live:/sledgehammer.iso rootfstype=auto ro liveimg rd_NO_LUKS rd_NO_MD rd_NO_DM provisioner.web={{.ProvisionerURL}} rebar.web={{.CommandURL}}"
RequiredParams:
  - "ntp_servers"
  - "provisioner-online"
  - "rebar-access_keys"
  - "rebar-machine_key"
Templates:
  - Name: pxelinux
    Path: pxelinux.cfg/default
    Contents: |
      DEFAULT discovery
      PROMPT 0
      TIMEOUT 10
      LABEL discovery
        KERNEL {{.Env.PathFor "tftp" .Env.Kernel}}
        INITRD {{.Env.JoinInitrds "tftp"}}
        APPEND {{.BootParams}}
        IPAPPEND
  - Name: elilo
    Path: elilo.conf
    Contents: |
      delay=2
      timeout=20
      verbose=5
      image={{.Env.PathFor "tftp" .Env.Kernel}}
      initrd={{.Env.JoinInitrds "tftp"}}
      append={{.BootParams}}"
  - Name: ipxe
    Path: default.ipxe
    Contents: |
      #!ipxe
      chain tftp://{{.ProvisionerAddress}}/${netX/ip}.ipxe && exit || goto sledgehammer
      :sledgehammer
      kernel {{.Env.PathFor "http" .Env.Kernel}} {{.BootParams}} BOOTIF=01-${netX/mac:hexhyp}
      initrd {{.Env.PathFor "http" .Env.Initrds[0]}}
      boot
'

sledgehammer='---
Name: "sledgehammer"
OS:
  Name: "sledgehammer/::SIG::"
  IsoFile: "sledgehammer-::SIG::.tar"
Kernel: "vmlinuz0"
Initrds:
  - "stage1.img"
BootParams: "rootflags=loop root=live:/sledgehammer.iso rootfstype=auto ro liveimg rd_NO_LUKS rd_NO_MD rd_NO_DM provisioner.web={{.ProvisionerURL}} rebar.web={{.CommandURL}} rebar.uuid={{.Machine.UUID}} rebar.install.key={{.Param \"rebar-machine_key\"}}"
RequiredParams:
  - "ntp_servers"
  - "provisioner-online"
  - "rebar-access_keys"
  - "rebar-machine_key"
Templates:
  - Name: "pxelinux"
    Path: "pxelinux.cfg/{{.Machine.HexAddress}}"
    Contents: |
      DEFAULT discovery
      PROMPT 0
      TIMEOUT 10
      LABEL discovery
        KERNEL {{.Env.PathFor "tftp" .Env.Kernel}}
        INITRD {{.Env.JoinInitrds "tftp"}}
        APPEND {{.BootParams}}
        IPAPPEND 2"
  - Name: "elilo"
    Path: "{{.Machine.HexAddress}}.conf"
    Contents: |
      delay=2
      timeout=20
      verbose=5
      image={{.Env.PathFor "tftp" .Env.Kernel}}
      initrd={{.Env.JoinInitrds "tftp"}}
      append={{.BootParams}}
  - Name: "ipxe"
    Path: "{{.Machine.Address}}.ipxe"
    Contents: |
      #!ipxe
      kernel {{.Env.PathFor "http" .Env.Kernel}} {{.BootParams}} BOOTIF=01-${netX/mac:hexhyp}
      {{ range $initrd := .Env.Initrds }}
      initrd {{$.Env.PathFor "http" $initrd}}
      {{ end }}
      boot
  - Name: "control.sh"
    Path: "{{.Machine.Path}}/control.sh"
    Contents: |
      #!/bin/bash
      # Copyright 2011, Dell
      #
      # Licensed under the Apache License, Version 2.0 (the "License");
      # you may not use this file except in compliance with the License.
      # You may obtain a copy of the License at
      #
      #  http://www.apache.org/licenses/LICENSE-2.0
      #
      # Unless required by applicable law or agreed to in writing, software
      # distributed under the License is distributed on an "AS IS" BASIS,
      # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
      # See the License for the specific language governing permissions and
      # limitations under the License.
      #

      # We get the following variables from start-up.sh
      # MAC BOOTDEV ADMIN_IP DOMAIN HOSTNAME HOSTNAME_MAC MYIP

      set -x
      shopt -s extglob
      export PS4="${BASH_SOURCE}@${LINENO}(${FUNCNAME[0]}): "
      cp /usr/share/zoneinfo/GMT /etc/localtime

      provisioner="{{.ProvisionerURL}}"
      api_server="{{.CommandURL}}"
      chef_rpm=chef-11.16.4-1.el6.x86_64.rpm

      # Set up just enough infrastructure to let the jigs work.
      # Allow client to pass http proxy environment variables
      echo "AcceptEnv http_proxy https_proxy no_proxy" >> /etc/ssh/sshd_config
      service sshd restart

      # Synchronize our date
      ntpdate "{{index (.Param "ntp_servers") 0}}"

      # Other gem dependency installs.
      cat > /etc/gemrc <<EOF
      :sources:
      {{ if .Param "provisioner-online" }}- http://rubygems.org/{{ end }}
      gem: --no-ri --no-rdoc --bindir /usr/local/bin
      EOF
      cp /etc/gemrc /root/.gemrc

      # Get the "right" version of Chef.  Eventually we should not hardcode this.
      [[ -f /tmp/$chef_rpm ]] || (
        cd /tmp
        curl -g -O "$provisioner/files/$chef_rpm"
        rpm -Uvh "./$chef_rpm"
      )

      mkdir -p /root/.ssh
      cat >/root/.ssh/authorized_keys <<EOF
      ### BEGIN GENERATED CONTENT
      {{ range $key := .Param "rebar-access_keys" }}{{$key}}{{ end }}
      ### END GENERATED CONTENT
      EOF

      # Mark us as alive.
      # Mark the node as alive.
      while ! rebar nodes update $REBAR_UUID "{\"alive\": true}"; do sleep 5; done

      # We are alive, and we should have a host entry created now.  Wait forever to do something.
      # The last line in this script must always be exit 0!!
      exit 0
'

if ! which rscli &>/dev/null; then
    echo "Cannot find rscli, need it to create Sledgehammer boot environments"
    exit 1
fi

SIG="fa8db28f5a64a54599afc0acbc5cf186e1ed57d8"
URL="http://opencrowbar.s3-website-us-east-1.amazonaws.com/sledgehammer/$SIG"

SS_URL="$URL/sledgehammer-${SIG}.tar"
[[ -f ${SS_URL##*/} ]] || curl -fgL -O "$SS_URL"

rscli isos upload "sledgehammer-${SIG}.tar" as "sledgehammer-${SIG}.tar"
sed "s/::SIG::/${SIG}/g" <<< "$discovery" | \
    if ! rscli bootenvs exists discovery; then
        rscli -F yaml bootenvs create -
    else
        rscli -F yaml bootenvs update discovery -
    fi

sed "s/::SIG::/${SIG}/g" <<< "$sledgehammer" | \
    if ! rscli bootenvs exists sledgehammer; then
        rscli -F yaml bootenvs create -
    else
        rscli -F yaml bootenvs update discovery -
    fi

