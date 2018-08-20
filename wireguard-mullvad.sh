
#/bin/bash
# Warning: Work in progress, still needs cleanup, but works on openwrt/ROOter.


#User Defined Variables:
endpoint_host="us2-wireguard"
mullvad_account='9199642831892489'


# Begin Script
uci set dhcp.@dnsmasq[0].server='8.8.8.8'
uci add_list dhcp.@dnsmasq[0].server='8.8.4.4'
uci delete network.wg0 2> /dev/null
uci commit
/etc/init.d/network reload
echo "nameserver 8.8.8.8" >> /etc/resolv.conf


local_private_key=`wg genkey`
local_public_key=`echo $local_private_key | wg pubkey`
mullvad_addresses=`curl https://api.mullvad.net/wg/ -d account="$mullvad_account"  --data-urlencode pubkey="$local_public_key"`
local_ipv4_address="`echo $mullvad_addresses | awk -F',' '{print $1}'`"
local_ipv6_address="`echo $mullvad_addresses | awk -F',' '{print $2}'`"


mullvad_vpn_servers=`curl https://www.mullvad.net/en/servers/#wireguard | grep -E '\-wireguard<|\=<' | awk -F'[><]' '{print $3}'`
endpoint_public_key=`echo "$mullvad_vpn_servers" | sed -n "/$endpoint_host/{n;p;}"`


endpoint_allowed_ips='0.0.0.0/0'
local_listen_port='51280'
endpoint_port='51280'
route_allowed_ips='1'
persistent_keepalive='25'

echo "endpoint_host=$endpoint_host"
endpoint_host=$(LC_ALL=C nslookup "$endpoint_host".mullvad.net 2>/dev/null  | sed -nr '/Name/,+1s|Address\ 1: *||p')

uci set network.wg0=interface
uci set network.wg0.proto='wireguard'
uci set network.wg0.listen_port="$local_listen_port"
uci set network.wg0.private_key="$local_private_key"
uci set network.wg0.addresses="$local_ipv4_address"
uci add_list network.wg0.addresses="$local_ipv6_address"

uci set network.@wireguard_wg0[0]=wireguard_wg0
uci set network.@wireguard_wg0[0].endpoint_allowed_ips="$allowed_ips"
uci set network.@wireguard_wg0[0].route_allowed_ips="$route_allowed_ips"
uci set network.@wireguard_wg0[0].endpoint_port="$endpoint_port"
uci set network.@wireguard_wg0[0].persistent_keepalive="$persistent_keepalive"
uci set network.@wireguard_wg0[0].public_key="$endpoint_public_key"
uci set network.@wireguard_wg0[0].endpoint_host="$endpoint_host"
uci set firewall.vpnzone.network='VPN wg0'
uci set dhcp.lan.dhcp_option='6,10.64.0.1'

uci set dhcp.@dnsmasq[0].nonwildcard='0'
uci set dhcp.@dnsmasq[0].server='10.64.0.1'
uci set dhcp.@dnsmasq[0].noresolv='1'
uci set network.wg0.auto='1'

uci commit
/etc/init.d/network restart

echo "endpoint_host=$endpoint_host"
echo "endpoint_public_key=$endpoint_public_key"
echo "local_private_key=$local_private_key"
echo "network.wg0.addresses=$local_ipv4_address"
echo "network.wg0.addresses=$local_ipv6_address"
