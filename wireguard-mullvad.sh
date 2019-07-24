#!/bin/ash
# ### Automatic setup of Wireguard for Mullvad
# ### Works on 3-hour demo accounts, just create a new one and re-run the script wiht new info to continue testing.
# ### Change endpoint and account number to your mullvad account, and run this script.
#
# Warning: Work in progress, still needs cleanup, but works on ROOter. Stock OpenWRT will need a change on the firewall zone.
# Capabilities of a MT7621A with 25% CPU left to spare, over LTE:
# With Wireguard: http://www.speedtest.net/result/7566309110.png
# Without Wireguard: http://www.speedtest.net/result/7566552283.png

#User Defined Variables:
endpoint_host="us2-wireguard"
mullvad_account='1940127723058875'


# Begin Script
if [ $(opkg list-installed | grep ca-bundle | wc -l) -lt 1 ]; then
    opkg update && opkg install curl ca-bundle
fi
#exit 0

uci set dhcp.@dnsmasq[-1].server='8.8.8.8'
uci add_list dhcp.@dnsmasq[-1].server='8.8.4.4'
uci delete network.@wireguard_wg0[-1] 2> /dev/null
uci delete network.@wireguard_wg0[-1] 2> /dev/null
uci delete network.@wireguard_wg0[-1] 2> /dev/null
uci delete network.wg0 2> /dev/null
uci delete network.wg0 2> /dev/null
uci commit
/etc/init.d/network reload
echo "nameserver 8.8.8.8" >> /etc/resolv.conf

local_private_key=$(wg genkey)
local_public_key=$(echo $local_private_key | wg pubkey)
mullvad_addresses=$(curl https://api.mullvad.net/wg/ -d account="$mullvad_account"  --data-urlencode pubkey="$local_public_key")
local_ipv4_address="$(echo $mullvad_addresses | awk -F',' '{print $1}')"
local_ipv6_address="$(echo $mullvad_addresses | awk -F',' '{print $2}')"


mullvad_vpn_servers=$(curl https://www.mullvad.net/en/servers/#wireguard | grep -E '\-wireguard<|\=<' | awk -F'[><]' '{print $3}')
endpoint_public_key=$(echo "$mullvad_vpn_servers" | sed -n "/$endpoint_host/{n;p;}")


endpoint_allowed_ips='0.0.0.0/0'
local_listen_port='51280'
endpoint_port='51280'
route_allowed_ips='1'
persistent_keepalive='25'

echo "endpoint_host=$endpoint_host"
endpoint_host=$(LC_ALL=C nslookup "$endpoint_host".mullvad.net 2>/dev/null  | sed -nr '/Name/,+1s|Address\ 1: *||p')

uci set network.wg0=interface
echo uci set network.wg0=interface
uci set network.wg0.proto='wireguard'
uci set network.wg0.listen_port="$local_listen_port"
uci set network.wg0.private_key="$local_private_key"
uci set network.wg0.addresses="$local_ipv4_address"
uci add_list network.wg0.addresses="$local_ipv6_address"
uci set network.wg0.auto='1'

uci add network wireguard_wg0
uci set network.@wireguard_wg0[-1]='wireguard_wg0'
uci set network.@wireguard_wg0[-1].allowed_ips="$endpoint_allowed_ips"
uci set network.@wireguard_wg0[-1].route_allowed_ips="$route_allowed_ips"
uci set network.@wireguard_wg0[-1].endpoint_port="$endpoint_port"
uci set network.@wireguard_wg0[-1].persistent_keepalive="$persistent_keepalive"
uci set network.@wireguard_wg0[-1].public_key="$endpoint_public_key"
uci set network.@wireguard_wg0[-1].endpoint_host="$endpoint_host"
uci set firewall.vpnzone.network='VPN wg0'
uci set dhcp.lan.dhcp_option='6,10.64.0.1'

uci set dhcp.@dnsmasq[-1].nonwildcard='0'
uci set dhcp.@dnsmasq[-1].server='10.64.0.1'
uci set dhcp.@dnsmasq[-1].noresolv='1'

uci commit
/etc/init.d/network reload

echo "endpoint_host=$endpoint_host"
echo "endpoint_public_key=$endpoint_public_key"
echo "local_private_key=$local_private_key"
echo "network.wg0.addresses=$local_ipv4_address"
echo "network.wg0.addresses=$local_ipv6_address"
echo "Rebooting in 5 seconds..."
sleep 5 && reboot
