#!/bin/bash

# Copyright (c) 2017 RackN Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

###
#  simple control script to build a DRP endpoint via CLI on a remote
#  target platform ... 
#
#  usage:   $0 --help
#
#   TODO:  * need to use correct URLs for getting content from official
#            repo locations
#          * fix private content get so that we don't have to pre-stage it
###

###
#  exit with error code and FATAL message
#  example:  xiterr 1 "error message"
###
function xiterr() { [[ $1 =~ ^[0-9]+$ ]] && { local _xit=$1; shift; } || local _xit=255; echo "FATAL: $*"; exit $_xit; }

###
#  set global XIT value if we receive an error exit code (anything 
#  besides a Zero)
###
function xit() { local _x=$1; (( $_x )) && (( XIT += _x )); }

[[ -f ./bin/color.sh ]] && source ./bin/color.sh
( type -t cprintf > /dev/null 2>&1 ) || function cprintf() { printf "%s" "$*"; }

# get our API_KEY and PROJECT_ID secrets
source ./private-content/secrets || xiterr 1 "unable to source './secrets' file "
[[ -z "$API_KEY"        || "$API_KEY"        == "insert_api_key_here" ]]         \
  && xiterr 1 "API_KEY is empty or unset ... bailing - check secrets file"
[[ -z "$PROJECT_ID"     || "$PROJECT_ID"     == "insert_project_id_here" ]]      \
  && xiterr 1 "PROJECT_ID is empty or unset ... bailing - check secrets file"
[[ -z "$RACKN_USERNAME" || "$RACKN_USERNAME" == "insert_rackn_username_here" ]]  \
  && xiterr 1 "RACKN_USERNAME is empty or unset ... bailing - check secrets file"

RACKN_AUTH="?username=${RACKN_USENAME}"

API="insert_api_key_here"
PROJECT="insert_project_id_here"
USERNAME="insert_rackn_username_here"

VER_DRP=${VER_DRP:-"stable"}
VER_CONTENT=${VER_CONTENT:-"stable"}
VER_PLUGINS=${VER_PLUGINS:-"tip"}
SSH_DRP_KEY=${SSH_DRP_KEY:-"5min-drp-ssh-key"}
SSH_NODES_KEY=${SSH_NODES_KEY:-"5min-nodes-ssh-key"}
MY_OS=${MY_OS:-"darwin"}
MY_ARCH=${MY_ARCH:-"amd64"}
DRP_OS=${DRP_OS:-"linux"}
DRP_ARCH=${DRP_ARCH:-"amd64"}
CREDS=${CREDS:-"--username=rocketskates --password=r0cketsk8ts"}
# do not use the CE based bootenvs for the Packet.net 5min demo
NODE_OS=${NODE_OS:-"centos-7.3.1611-install"}  # ubuntu-16.04-install

CURL="curl -sfSL"
DRPCLI="drpcli"
XIT=0

# add HOME/bin to path if it's not there already
[[ ":$PATH:" != *":$HOME/bin:"* ]] && PATH="$HOME/bin:${PATH}"
[[ ":$PATH:" != *":`pwd`/bin:"* ]] && PATH="`pwd`/bin:${PATH}"

function usage() {

cat <<END_USAGE
USAGE:  $0 [arguments]
WHERE: arguments are as follows:

    help | usage           this help statement

    install-terraform      installs terraform locally
    install-secrets        installs API and PROJECT secrets for Terraform files
    ssh-keys               generates new ssh keys, REMOVES existing keys first
    set-drp-endpoint <ID>  sets the drp-nodes.tf endpoint information 
                           for Terraform
    get-drp-local          installs DRP locally
    get-drp-cc             installs DRP *community* content 
    get-drp-plugins        installs DRP Packet Plugins
    drp-install <ID>       install DRP and basic content as identified by <ID>
    remote-content <ID>    do 'get-drp-cc' and 'get-drp-plugins' on remote <ID>
    drp-setup <ID>         perform content and plugins setup on <ID> endpoint

    get-drp-id             get the DRP endpoint server ID
    get-address <ID>       get the IP address of new DRP server identified by <ID>
    ssh <ID> [COMMANDS]    ssh to the IP address of DRP server identified by <ID>
    scp <ID> [FILES]       ssh to the IP address of DRP server identified by <ID>

    cleanup                WARNING WARNING WARNING

CLEANUP:  WARNING - cleanup will NUKE things - like private SSH KEY (and more)  !!!

  NOTES:  * 'get-drp-cc' and 'get-drp-plugins' run on the local control host
            'remote-content' runs the content pull FROM the <ID> endpoint
            ONLY run 'get-drp-*' _OR_ 'remote-content' - NOT both

          * get-drp-id gets the ID of the DRP endpoint server - suggest adding
            to your environment variables like:
               #  export DRP=\`$0 get-drp-id\`

          * <ID> is the ID of the DRP endpoint that is created by terraform 

          * you can override built in defaults by setting the following variables:
             SSH_DRP_KEY  SSH_NODES_KEY  MY_OS    MY_ARCH      DRP_OS      DRP_ARCH
             CREDS        NODE_OS        VER_DRP  VER_CONTENT  VER_PLUGINS

END_USAGE
} # end usaage()

# ssh function
#   ARGv1 shouuld be terraform ID of target
#   remains args are commands to execute on remote side
#   global var SSH_DRP_KEY must be set to private key
function my_ssh() {
  [[ -z "$1" ]] && xiterr 1 "Need DRP endpoint ID as argument 1"
  local _target=`$0 get-address $1`
  shift 1

  [[ ! -r "$SSH_DRP_KEY" ]] && xiterr 1 "ssh key file ('$SSH_DRP_KEY') not readable"
  ssh -x -i ${SSH_DRP_KEY} root@$_target "$*" 
  xit $?
}

# copy files to remote target
#   ARGv1 shouuld be terraform ID of target
#   remains args are files to SCP
#   global var SSH_DRP_KEY must be set to private key
#
# TODO:  should switch to using rsync 
function my_copy() {
  [[ -z "$1" ]] && xiterr 1 "Need DRP endpoint ID as argument 1"
  local _target=`$0 get-address $1`
  shift 1

  [[ ! -r "$SSH_DRP_KEY" ]] && xiterr 1 "ssh key file ('$SSH_DRP_KEY') not readable"
  scp -i ${SSH_DRP_KEY} $* root@$_target: 
  xit $?
}

###
#  accept as ARGv1 a sha256 check sum file to test
###
function check_sum() {
  local _sum=$1
  [[ -z "$_sum" ]] && xiterr 1 "no check sum file passed to check_sum()"
  [[ ! -r "$_sum" ]] && xiterr 1 "unable to read check sum file '$_sum'"
  local _platform=`uname -s`

  case $_platform in
    Darwin) shasum -a 256 -c $_sum; xit $? ;;
    Linux)  sha256sum -c $_sum; xit $? ;;
    *) xiterr 2 "unsupported platform type '$_platform'"
  esac

  (( $XIT )) && xiterr 9 "checksum failed for '$_sum'"
}

function prereqs() {
  local _pkgs
  local _yq="https://gist.githubusercontent.com/earonesty/1d7cb531bb8fff8c228b7710126bcc33/raw/e250f65764c448fe4073a746c4da639d857c9e6c/yq"
  # test for our prerequisites here and add them to _pkgs parameter if missing
  mkdir -p $HOME/bin
  ( which unzip > /dev/null 2>&1 ) || _pkgs="unzip $_pkgs"
  ( which jq > /dev/null 2>&1 ) || _pkgs="jq $_pkgs"
  ( which yq > /dev/null 2>&1 ) || { $CURL $_yq -o $HOME/bin/yq; chmod 755 $HOME/bin/yq; }

  [[ -z "$_pkgs" ]] && return
	os_info

	case $_OS_FAMILY in
		rhel)   sudo yum -y install $_pkgs; xit $? ;;
		debian) sudo apt -y install $_pkgs; xit $? ;;
    darwin) ;;
    *)  xiterr 4 "unsupported _OS_FAMILY ('$_OS_FAMILY') in prereqs()" ;;
	esac

  (( $XIT )) && xiterr 1 "prerequisites failed ('$_pkgs')"
}

# set our global _OS_* variables for re-use
function os_info() {
	# Figure out what Linux distro we are running on.
	# set these globally for use outside of the script
	export _OS_TYPE= _OS_VER= _OS_NAME= _OS_FAMITLY=

	if [[ -f /etc/os-release ]]; then
    source /etc/os-release
    _OS_TYPE=${ID,,}
    _OS_VER=${VERSION_ID,,}
	elif [[ -f /etc/lsb-release ]]; then
    source /etc/lsb-release
    _OS_VER=${DISTRIB_RELEASE,,}
    _OS_TYPE=${DISTRIB_ID,,}
	elif [[ -f /etc/centos-release || -f /etc/fedora-release || -f /etc/redhat-release ]]; then
    for rel in centos-release fedora-release redhat-release; do
        [[ -f /etc/$rel ]] || continue
        _OS_TYPE=${rel%%-*}
        _OS_VER="$(egrep -o '[0-9.]+' "/etc/$rel")"
        break
    done

    if [[ ! $_OS_TYPE ]]; then
        echo "Cannot determine Linux version we are running on!"
        exit 1
    fi
	elif [[ -f /etc/debian_version ]]; then
    _OS_TYPE=debian
    _OS_VER=$(cat /etc/debian_version)
	elif [[ $(uname -s) == Darwin ]] ; then
    _OS_TYPE=darwin
    _OS_VER=$(sw_vers | grep ProductVersion | awk '{ print $2 }')
	fi
	_OS_NAME="$_OS_TYPE-$_OS_VER"

	case $_OS_TYPE in
    centos|redhat|fedora) _OS_FAMILY="rhel";;
    debian|ubuntu) _OS_FAMILY="debian";;
    *) _OS_FAMILY=$_OS_TYPE;;
	esac
} # end os_family()

prereqs 

# we're going to stuff some binaries in the local bin/ path
PATH=`pwd`/bin:$PATH

case $1 in 
  usage|--usage|help|--help|-h)
    usage
    ;;

  install-secrets)
      sed -i.bak                                           \
        -e "s/insert_api_key_here/$API_KEY/g"              \
        -e "s/insert_project_id_here/$PROJECT_ID/g"        \
        vars.tf
      if (( $? ))
      then
        xiterr 1 "unable to install secrets to vars.tf"
      else
        echo "Secrets installed to vars.tf ..."
      fi
    ;;

  get-drp-local)
    rm -rf dr-provision-install
    mkdir dr-provision-install
    cd dr-provision-install
    set -x
    $CURL https://github.com/digitalrebar/provision/releases/download/${VER_DRP}/dr-provision.zip -o dr-provision.zip
    $CURL https://github.com/digitalrebar/provision/releases/download/${VER_DRP}/dr-provision.sha256 -o dr-provision.sha256
    set +x
    check_sum dr-provision.sha256 

    unzip dr-provision.zip
    cd ..

    [[ -f "`pwd`/bin/drpcli" ]] && rm -f `pwd`\/bin/drpcli
    ln -s `pwd`/dr-provision-install/bin/${MY_OS}/${MY_ARCH}/drpcli `pwd`/bin/drpcli
    $DRPCLI version || xiterr 1 "failed to install DRP endpoint in bin/ directory"
    ;;

  install-terraform)
    # get, and install terraform
    mkdir -p bin
    mkdir -p tmp
    cd tmp
    # make a reasonable attempt at getting the latest version of Terraform
    TF_VER=`curl -s https://checkpoint-api.hashicorp.com/v1/check/terraform | jq -r -M '.current_version'`
    # see:  https://github.com/hashicorp/terraform/issues/9803
    # thank you Apple for providing a hobbled version of 'sort' that doesn't contain "-V" 
    # (version sorting) capabilities
    #`curl -s https://releases.hashicorp.com/terraform/   \
    #  | grep terraform_                                         \
    #  | egrep -v "rc|beta"                                      \
    #  | egrep "_[1-9]\.[0-9].*|0\.[1-9][0-9]\."                 \
    #  | sed 's/\(^.*>\)\(terraform_.*\..*\..*\)\(<\/.*$\)/\2/g' \
    #  | sort -n                                                 \
    #  | tail -1                                                 \
    #  | sed 's/terraform_//g'`
    wget -O tf.zip https://releases.hashicorp.com/terraform/${TF_VER}/terraform_${TF_VER}_${MY_OS}_${MY_ARCH}.zip && unzip tf.zip
    mv terraform ../bin/ && chmod 755 ../bin/terraform
    rm tf.zip
    cd ..
    rm -rf tmp

    # since the terraform-provider-packet plugin requires GO to compile - we are pre requiring
    # you to pre-stage it in the private-content directory.  If you 
    # have GO 1.9.0 or newer - you can get/compile/install it with:
    #    go get -u github.com/terraform-providers/terraform-provider-packet
    # copy the plugin out of the generated $HOME/go/bin/ (usually) directory to 
    # the private-content/ directory

    PRIV_TPP="private-content/terraform-provider-packet"
    [[ -f "$PRIV_TPP" ]] && cp "$PRIV_TPP" bin/
    
    TF_PLUG="`pwd`/bin/terraform-provider-packet"
    [[ ! -r "${TF_PLUG}" ]] && xiterr 4 "Terraform packet plugin not found ('$TF_PLUG')"
    if [[ -f $HOME/.terraformrc ]] 
    then
      echo "Backing up $HOME/.terraformrc to $HOME/.terraformrc.5min-backup"
      cp $HOME/.terraformrc $HOME/.terraformrc.5min-backup
    fi
    echo "providers { packet = \"${TF_PLUG}\" }" >> $HOME/.terraformrc
    terraform init || xiterr 1 "terraform init failed"
    ;;

  set-drp-endpoint)
    [[ -z "$2" ]] && xiterr 1 "Need DRP endpoint ID as argument 2"
    ADDR=`$0 get-address $2`
    ( sed -i.bak 's+\(^chain http://\)\(.*\)\(/default.ipxe.*$\)+\1'${ADDR}':8091\3+g' drp-nodes.tf ) \
      && echo "DRP endpoint set in 'drp-nodes.tf' successfully: " \
      || xiterr 1 "DRP endpoint set FAILED for 'drp-nodes.tf'"
    _chain=`cprintf $cyan $(grep "^chain " drp-nodes.tf)`
    echo "  ipxe -->  $_chain"
    xit $?
    ;;

  get-drp-cc)
    # community content is installed via install.sh of DRP - unless "--nocontent" is specified
    echo ""
    rm -rf dr-provision-install/drp-community-content.*
    mkdir -p dr-provision-install
    cd dr-provision-install

    # community contents
    # it appears it's distributed by default now ... 
    $CURL \
      https://github.com/digitalrebar/provision-content/releases/download/${VER_CONTENT}/drp-community-content.yaml \
      -o drp-community-content.yaml
    $CURL \
      https://github.com/digitalrebar/provision-content/releases/download/${VER_CONTENT}/drp-community-content.sha256 \
      -o drp-community-content.sha256

    check_sum drp-community-content.sha256
    cd ..

    ;;

  # get-drp-plugins relies on private-content for the RackN specific conent 
  # this is VERY different from the get-drp-cc (Community Content)
  get-drp-plugins)
#    [[ ! -r private-content/drp-rack-plugins-${DRP_OS}-${DRP_ARCH}.zip ]] && xiterr 1 "missing private-content plugins"

    rm -rf dr-provision-install/drp-rack-plugins-${DRP_OS}-${DRP_ARCH}.*
    mkdir -p dr-provision-install
    cd dr-provision-install

    # packet helper content
    $CURL \
      https://qww9e4paf1.execute-api.us-west-2.amazonaws.com/main/catalog/content/packet${RACKN_AUTH} \
      -o drp-content-packet.json
    ls -l drp-content-packet.json

     $CURL \
       https://qww9e4paf1.execute-api.us-west-2.amazonaws.com/main/catalog/plugins/packet-ipmi${RACKN_AUTH} \
       -o drp-plugin-packet-ipmi.json
    ls -l drp-plugin-packet-ipmi.json

    # get our packet-ipmi provider plugin location 
    PACKET_URL="https://qww9e4paf1.execute-api.us-west-2.amazonaws.com/main/catalog/plugins/packet-ipmi${RACKN_AUTH}"
    PART=`$CURL $PACKET_URL | jq ".$DRP_ARCH.$DRP_OS"`
    BASE=`$CURL $PACKET_URL | jq '.base'`
    # download the plugin
    $CURL $BASE/$PART -o drp-plugin-packet-ipmi

# currently these plugins are closed to community - so you MUST obtain this
# with authenticated gitlab account, and copy to the private-content directory
#    $CURL \
#      https://github.com/rackn/provision-plugins/releases/download/${VER_PLUGINS}/drp-rack-plugins-${DRP_OS}-${DRP_ARCH}.sha256 \
#      -o drp-rack-plugins-${DRP_OS}-${DRP_ARCH}.sha256
#    $CURL  \
#      https://github.com/rackn/provision-plugins/releases/download/${VER_PLUGINS}/drp-rack-plugins-${DRP_OS}-${DRP_ARCH}.zip \
#      -o drp-rack-plugins-${DRP_OS}-${DRP_ARCH}.zip

# moved to CURL grab of the plugin with authenticated username
#    cp ../private-content/drp-rack-plugins* ./
#    check_sum drp-rack-plugins-${DRP_OS}-${DRP_ARCH}.sha256

#		rm -rf plugins
#    mkdir -p plugins
#		cd plugins
#		unzip ../drp-rack-plugins-${DRP_OS}-${DRP_ARCH}.zip
#    check_sum sha256sums

    cd ../..
    ;;

  remote-content)
    [[ -z "$2" ]] && xiterr 1 "Need DRP endpoint ID as argument 2"
    ADDR=`$0 get-address $2`

    # drp-community-content is  installed by default (unless '--nocontent' specified)
    # do not attempt to install it again
    # $0 ssh $2 "hostname; $0 get-drp-cc $2; $0 get-drp-plugins $2; bash -x $0 drp-setup $2"
    CMD="hostname; ./bin/control.sh get-drp-plugins $2; bash -x ./bin/control.sh drp-setup $2"
    my_ssh $2 "$CMD"
    xit $?
    ;;

  ssh|scp|copy)
    [[ -z "$2" ]] && xiterr 1 "Need DRP endpoint ID as argument 2"
    _cmd=$1
    shift 1

    case $_cmd in
      ssh) my_ssh $*
        ;;
      copy|scp) my_copy $*
        ;;
    esac
    ;;

  ssh-keys)
    # remove keys if they exist already 
    [[ -f "${SSH_DRP_KEY}" ]] && rm -f ${SSH_DRP_KEY}
    [[ -f "${SSH_DRP_KEY}.pub" ]] && rm -f ${SSH_DRP_KEY}.pub
    ssh-keygen -t rsa -b 4096 -C "5min-drp-ssh-key" -P "" -f ${SSH_DRP_KEY}
    xit $?

    if [[ "$SSH_DRP_KEY != "$SSH_NODES_KEY ]]
    then
      [[ -f "${SSH_NODES_KEY}" ]] && rm -f ${SSH_NODES_KEY}
      [[ -f "${SSH_NODES_KEY}.pub" ]] && rm -f ${SSH_NODES_KEY}.pub
      ssh-keygen -t rsa -b 4096 -C "5min-nodes-ssh-key" -P "" -f ${SSH_NODES_KEY}
      xit $?
    fi

    ;;

  get-drp-id)
    terraform plan | grep packet_device.5min-drp | awk ' { print $NF } ' | sed 's/)//'
    xit $?
    ;;

  get-address)
    [[ -z "$2" ]] && xiterr 1 "Need DRP endpoint ID as argument 2"

    [[ ! -r terraform.tfstate ]] && xiterr 3 "terraform.tfstate not readable, did you run 'terraform apply'?"
    cat terraform.tfstate \
      | jq -r '.modules[].resources."packet_device.5min-drp".primary.attributes."network.0.address"'
    xit $?
#    $CURL -X GET --header "Accept: application/json" \
#      --header "X-Auth-Token: ${API_KEY}"              \
#      "https://api.packet.net/devices/${2}"            \
#      | jq -rcM '.ip_addresses[0].address'

    ;;

  drp-install)
    [[ -z "$2" ]] && xiterr 1 "Need DRP endpoint ID as argument 2"
    A=`$0 get-address $2`

    echo "Pushing helper content to remote DRP endpoint ... "
    echo "           ID :: '$2'"
    echo "   IP Address :: '$A'"
    my_ssh $2 "mkdir -p bin"
    my_copy $2 -r bin/drp-install.sh terraform.tfstate $0 private-content/ 

    echo "Installing DRP endpoint service on remote host ... "
    my_ssh $2 "mv *.sh bin/; chmod 755 bin/*.sh; VER_DRP=${VER_DRP} ./bin/drp-install.sh"
    ;;

  # horri-bad hack to fix bug w/ stages not eval as valid
  # intended to be run on remote DRP endpoint
  fix-stages-bug)

    URL="https://qww9e4paf1.execute-api.us-west-2.amazonaws.com/main/catalog/content/packet${RACKN_AUTH}"
    CONTENT="dr-provision-install/drp-content-packet.json"
    CONTENT_NAME=`cat $CONTENT | jq -r '.meta.Name'`
    set -x
    $DRPCLI $ENDPOINT contents destroy "$CONTENT_NAME"
    $DRPCLI $ENDPOINT contents create - < $CONTENT
    set +x
    ;;
  # sets up the RackN specific content packs on a DRP endpoint - VERY different
  # from CC (community content)
  drp-setup)
    _ext=""
    [[ -z "$2" ]] && xiterr 1 "Need DRP endpoint ID as argument 2"
    ADDR=`$0 get-address $2`

    ENDPOINT="--endpoint=https://$ADDR:8092 $CREDS"

    # get content
    URLS="
    https://qww9e4paf1.execute-api.us-west-2.amazonaws.com/main/catalog/content/os-linux${RACKN_AUTH}
    https://qww9e4paf1.execute-api.us-west-2.amazonaws.com/main/catalog/content/os-discovery${RACKN_AUTH}
    https://qww9e4paf1.execute-api.us-west-2.amazonaws.com/main/catalog/content/packet${RACKN_AUTH}
    "
    for URL in $URLS
    do
      CONTENT_NAME="content-`basename $URL`.json"
      set -x
      $CURL $URL -o dr-provision-install/$CONTENT_NAME
      set +x
    done

    # install content 
    for CONTENT in dr-provision-install/*content*.[jy][sa]*
    do
      _ext=${CONTENT#*.}
      case $_ext in 
        json)
          CONTENT_NAME=`cat $CONTENT | jq -r '.meta.Name'`
          ;;
        yaml|yml)
          CONTENT_NAME=`cat $CONTENT | yq -r '.meta.Name'`
          ;;
      esac

      if ( $DRPCLI $ENDPOINT contents exists "$CONTENT_NAME" > /dev/null 2>&1 )
      then
        set -x
        $DRPCLI $ENDPOINT contents destroy "$CONTENT_NAME"
        set +x
      fi

      set -x
      $DRPCLI $ENDPOINT contents create - < $CONTENT
      set +x
    done  

    # install packet-ipmi plugin
    for PLUGIN in dr-provision-install/drp-plugin-*
    do
      PLUG_NAME=`basename $PLUGIN | sed 's/^drp-plugin-//'`

      if ( $DRPCLI $ENDPOINT plugin_providers exists $PLUG_NAME > /dev/null 2>&1 )
      then
        set -x
        $DRPCLI $ENDPOINT plugin_providers destroy $PLUG_NAME
        set +x
      fi

      set -x
      $DRPCLI $ENDPOINT plugin_providers upload $PLUGIN as $PLUG_NAME
      set +x
    done

#    if ( $DRPCLI $ENDPOINT plugins exists packet-ipmi > /dev/null 2>&1 )
#    then
#      set -x
#      $DRPCLI $ENDPOINT plugins destroy packet-ipmi
#      set +x
#    fi

    cat <<EOFPLUGIN > private-content/packet-ipmi-plugin-create.json
    {
      "Available": true,
      "Name": "packet-ipmi",
      "Description": "Packet IPMI API Key",
      "Provider": "packet-ipmi",
      "Params": { "packet/api-key": "$API_KEY" }
    }
EOFPLUGIN

    if ( $DRPCLI $ENDPOINT plugins exists "packet-ipmi" > /dev/null 2>&1 )
    then
      $DRPCLI $ENDPOINT plugins destroy "packet-ipmi"
    fi
    $DRPCLI $ENDPOINT plugins create - < private-content/packet-ipmi-plugin-create.json
    # set up the packet stage map 
    # create stagemap JSON (NODE_OS:  ubuntu-16.04-install)
	  cat <<EOFSTAGE > private-content/stagemap-create.json
    {
      "Available": true,
      "Description": "5min-packet-map",
      "Name": "global",
      "Params": {
          "change-stage/map": {
            "discover": "packet-discover:Success",
            "packet-discover": "${NODE_OS}:Reboot",
            "packet-ssh-keys": "complete-nowait:Success",
            "${NODE_OS}": "packet-ssh-keys:Success"
        }
      }
    }
EOFSTAGE

    if ( $DRPCLI $ENDPOINT profiles exists global > /dev/null 2>&1 )
    then
      $DRPCLI $ENDPOINT profiles destroy global
    fi
    $DRPCLI $ENDPOINT profiles create - < private-content/stagemap-create.json

    # upload our isos
    UPLOADS="sledgehammer $NODE_OS"
    for UPLOAD in $UPLOADS
    do
    $DRPCLI $ENDPOINT bootenvs exists $UPLOAD \
      && $DRPCLI $ENDPOINT bootenvs uploadiso $UPLOAD \
      || echo "bootenv '$UPLOAD' doesn't exist, not uploading ISO"
    done

    # verify we have our stages/bootenvs available before setting the defaults for them
    ( $DRPCLI $ENDPOINT stages exists discover > /dev/null 2>&1 ) || xiterr 9 "default stage ('discover') doesn't exist"
    ( $DRPCLI $ENDPOINT bootenvs exists sledgehammer > /dev/null 2>&1 ) || xiterr 9 "default BootEnv ('sledgehammer') doesn't exist"
    ( $DRPCLI $ENDPOINT bootenvs exists discovery > /dev/null 2>&1 ) || xiterr 9 "unknown BootEnv ('discovery') doesn't exist"

    # set our default Stage, Default Boot Enviornment, and our Unknown Boot Environment
    $DRPCLI $ENDPOINT prefs set defaultStage discover defaultBootEnv sledgehammer unknownBootEnv discovery
    ;;

  cleanup)
    ###
    #  brain dead cleanup script ... I hope you know what you're doing ...
    ###
    if [[ -z "$2" ]]
    then
      N=8
      echo -n "Going to nuke stuff (like SSH KEYS !!) in $N seconds [ Ctrl-C to cancel ] : "
      while (( N > 0 )); do echo -n " $N "; sleep 1; (( N-- )); done
      echo -n " ... "; sleep 1; echo "Bang!"
    fi
    echo ""

    set -x
    rm -f ${SSH_DRP_KEY} ${SSH_DRP_KEY}.pub
    rm -f ${SSH_NODES_KEY} ${SSH_NODES_KEY}.pub
    rm -f drpcli dr-provision-install
    rm -rf tmp 
    rm -rf bin/terraform bin/drpcli bin/dr-provision bin/terraform-provider-packet bin/yq

    sed -i.bak                                                           \
      -e 's/^\(API="\)\(.*\)\("\)/\1insert_api_key_here\3/g'             \
      -e 's/^\(PROJECT="\)\(.*\)\("\)/\1insert_project_id_here\3/g'      \
      -e 's/^\(USERNAME="\)\(.*\)\("\)/\1insert_rackn_username_here\3/g' \
      private-content/secrets
    sed -i.bak                                                                \
      -e 's/\(^.*packet_api_key.*"\)\(.*\)\(".*$\)/\1insert_api_key_here\3/g' \
      -e 's/\(^.*project_id.*"\)\(.*\)\(".*$\)/\1insert_project_id_here\3/g'  \
      vars.tf

    sed -i.bak                                                                          \
      's+\(^chain http://\)\(.*\)\(/default.ipxe\)+\1drp_endpoint_address_and_port\3+g' \
      drp-nodes.tf

    find private-content/ -type f | grep -v "/secrets$" | xargs rm -rf 
    set +x

    [[ -f $HOME/.terraformrc.5min-backup ]] && mv $HOME/.terraformrc.5min-backup $HOME/.terraformrc

    echo "No terraform actions taken - please nuke resources via terraform ... "
    echo "       Suggest:  terraform destroy --force"
    echo "                 $0 extra-cleanup"
    ;;

  extra-cleanup)
    echo "performing extra cleanup tasks .... "
    set -x
    rm -rf *bak private-content/*bak terraform.tfstate* $HOME/.terraformrc ./.terraform 
    rm -rf dr-provision-install
    set +x
    ;;

  *) 
    $0 usage
    xiterr 99 "unknown option(s) '$*'"
    ;;
esac

exit $XIT
