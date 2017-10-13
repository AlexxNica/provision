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

################## 
################## SEE README for details on usage of this script .... 
################## 

[[ -f ./bin/color.sh ]] && source ./bin/color.sh
( type -t cprintf > /dev/null 2>&1 ) || function cprintf() { printf "%s" "$*"; }

[[ ":$PATH:" != *":`pwd`/bin:"* ]] && PATH="`pwd`/bin:${PATH}"

cloudia.sh

CONFIRM=${CONFIRM:-"yes"}

function confirm() {
  local _sep="--------------------------------------------------------------------------------"
  local _err
  local _wait
  local _action=`cprintf "$bold$underline" "ACTION"`
  local _msg=`cprintf $green "$*"`
  local _default=`cprintf $cyan "<Enter>"`
  local _failed=`cprintf $red "FAILED"`
  local _success=`cprintf $green "Success... "`
  local _skipping=`cprintf $yellow "Skipping..."`

  echo ""
  cprintf $magenta "${_sep}\n"
  echo "$_action :: $_msg"

  if [[ $CONFIRM == "yes" ]] 
  then
    echo -n "Run next step?  [ $_default | No | Ctrl-C ]  "
    read _wait
  fi

  if [[ "$_wait" =~ [Nn].* ]] 
  then
    echo "$_skipping"
    echo "$_sep"
    return
  else
    echo "$_sep"
    echo ""
    $*
    _err=$?

    (( $_err )) && echo "$_failed" || echo "$_success"
  fi
}

###
#  we assume you've checked out the examples/5min-drp/ directory from the
#  Digital Rebar Provision repo ... do something like:
###
# echo "ACTION: Cloning GIT repo contents ... "
# git clone -n https://github.com/digitalrebar/provision.git --depth=1
# cd provision
# git checkout HEAD examples/5min-drp
# cd ..
# mv examples/5min-drp $HOME/
# cd $HOME/5min-drp

#
# vim private-content/secrets
if [[ "$USER" == "shane" ]]
then
  echo "<<SHANE>> Staging terraform plugins, private content, and secrets ... "
  set -x
  cp $HOME/private-content/drp-rack-plugins* ./private-content/
  cp $HOME/private-content/terraform-provider-packet bin/
  cp $HOME/private-content/secrets ./private-content
  set +x
fi

# installs terraform locally
confirm control.sh install-terraform    

# installs API and PROJECT secrets for Terraform files
confirm control.sh install-secrets      

# removes ssh keys if exists and generates new keys
confirm control.sh ssh-keys             

# apply our SSH keys 
confirm terraform apply -target=packet_ssh_key.5min-drp-ssh-key
confirm terraform apply -target=packet_ssh_key.5min-nodes-ssh-key

# build our DRP server
confirm terraform apply -target=packet_device.5min-drp


# view our completed plan status -- NOTE the "5min-nodes"
# do NOT get applied until after 5min-drp is finished 
confirm terraform plan                    

# installs DRP locally for CLI commands
confirm control.sh get-drp-local        

# get the DRP endpoint server ID
confirm control.sh get-drp-id           

# assign our ID to DRP variable for easy reuse below
confirm export DRP=`control.sh get-drp-id`

# get our DRP Endpoint IP Address to manipulate our SSH Host Keys
confirm export ADDR=`control.sh get-address $DRP`

# remove any existing host keys
confirm ssh-keygen -R $ADDR

# install the newly built DRP Endpoint host key
confirm "ssh-keyscan -H $ADDR >> $HOME/.ssh/known_hosts 2> /dev/null"

# install DRP and basic content as identified by <ID>
confirm control.sh drp-install $DRP     

case $1 in 
  local)
    echo "Installing content to DRP endpoint ('$DRP') from local system (push to endpoint)..."
    # installs DRP community content locally
    confirm control.sh get-drp-cc
    # installs DRP Packet Plugins
    confirm control.sh get-drp-plugins      
    # perform content and plugins setup on <ID> endpoint
    confirm control.sh drp-setup $DRP
  ;;
  remote|*)
    echo "Installing content from DRP endpoint ('$DRP') (pull from endpoint)..."
    # runs 'get-drp-cc', 'get-drp-plugins', and 'drp-setup' on remote <ID>
    echo ""
    cprintf $bold "   SSH to remote DRP, stop, restart in foreground ... ? "
    cprintf $bold "   Maybe launch UI to show empty content too ... ? "
    cprintf $bold "   https://rackn.github.io/provision-ux/#/e/${ADDR}:8092/system "
    echo ""
    confirm control.sh remote-content $DRP  
    echo ""
    cprintf $cyan "NOTICE:"
    echo "         Errors may be 'normal' - ISOs, Kernel, and InitRDs are "
    echo "         normal as the content has not yet been pused to the DRP"
    echo "         endpoint.  Other errors should be investigated."
    echo ""
  ;;
esac

# inject our DRP endpoint address in to the drp-nodes.tf terraform file
confirm control.sh set-drp-endpoint $DRP

# bug in Stages causes stage "discover" to be marked bad, only way to 
# get it to re-eval as good is to force delete content, and re-add which
# triggers refresh of stage checks
confirm control.sh ssh $DRP "bin/control.sh fix-stages-bug $DRP"

# bring up our DRP target nodes:
confirm terraform apply -target=packet_device.5min-nodes

# helper functions ... not used in demo
#control.sh get-address <ID>     # get the IP address of new DRP server identified by <ID>
#control.sh ssh <ID> [COMMANDS]  # ssh to the IP address of DRP server identified by <ID>
#control.sh scp <ID> [FILES]     # ssh to the IP address of DRP server identified by <ID>



