#!/bin/bash
#.USAGE
# To start, run:
# wget https://raw.githubusercontent.com/danielewood/sierra-wireless-modems/master/autoflash-7455.sh && sudo bash autoflash-7455.sh

#.SYNOPSIS
# - Only for use on Ubuntu 18.04 LTS LiveUSB
# - Changes all models of EM7455/MC7455 Modems to the Generic Sierra Wireless VID/PID
# - Flashes the Current Generic Firmware as of 2018-09-14

#.DESCRIPTION
# - Only for use on Ubuntu 18.04 LTS LiveUSB
# - All Needed Packages will Auto-Install
# - Sets MBIM Mode with AT Commands Access 
# - Changes all models of EM74XX/MC74XX Modems to the Generic Sierra Wireless VID/PID
# - Clears Band Restrictions and Places Modem in LTE only mode.
# - Flashes the Current Generic Firmware as of 2018-09-14

#.NOTES
# License: The Unlicense / CCZero / Public Domain
# Author: Daniel Wood / https://github.com/danielewood

#.LINK
# https://github.com/danielewood/sierra-wireless-modems

#.VERSION
# Version: 20180914

BLUE='\033[0;34m'
NC='\033[0m' # No Color

if [ "$EUID" -ne 0 ]
    then echo "Please run with sudo"
    exit
fi

lsbrelease=`lsb_release -c | awk '{print $2}'`
if [ "$lsbrelease" != "bionic" ]
    then echo "Please run on Ubuntu 18.04 LTS"
    lsb_release -a
    exit
fi

printf "${BLUE}---${NC}\n"
echo 'Searching for EM7455/MC7455 USB modems...'
modemcount=`lsusb | grep -i -E '1199:9071|1199:9079|413C:81B6' | wc -l`
while [ $modemcount -eq 0 ]
do
    printf "${BLUE}---${NC}\n"
    echo "Could not find any EM7455/MC7455 USB modems"
    echo 'Unplug and reinsert the EM7455/MC7455 USB connector...'
    modemcount=`lsusb | grep -i -E '1199:9071|1199:9079|413C:81B6' | wc -l`
    sleep 3
done

printf "${BLUE}---${NC}\n"
echo "Found EM7455/MC7455: 
`lsusb | grep -i -E '1199:9071|1199:9079|413C:81B6'`
"

if [ $modemcount -gt 1 ]
then 
    printf "${BLUE}---${NC}\n"
    echo "Found more than one EM7455/MC7455, remove the one you dont want to flash and try again."
    exit
fi

# Stop modem manager to prevent AT command spam and allow firmware-update
printf "${BLUE}---${NC}\n"
echo 'Stoping modem manager to prevent AT command spam and allow firmware-update, this may take a minute...'
systemctl stop ModemManager
systemctl disable ModemManager

printf "${BLUE}---${NC}\n"
echo "Installing all needed prerequisites..."
apt-get update
# need make and GCC for compiling perl modules
apt-get install make gcc curl -y
# Use cpan to install/compile all dependencies needed by swi_setusbcomp.pl
yes | cpan install UUID::Tiny IPC::Shareable JSON

# apt-get will fail to download minicom/qmi-utilities on LiveCD/LiveUSB without adding repositories
# Also, if you add security.ubuntu.com bionic main universe, you'll get an older version of libqmi (1.18)
# So we'll pull the .deb files directly, grepping repo to always pull newest version
deb_minicom=`curl http://security.ubuntu.com/ubuntu/pool/universe/m/minicom/ 2>/dev/null | grep -Eo '"minicom.*amd64.deb"' | tail -n1 | sed 's/\"//g'`
deb_libqmi_glib5=`curl http://security.ubuntu.com/ubuntu/pool/main/libq/libqmi/ 2>/dev/null | grep -Eo '"libqmi-glib5.*amd64.deb"' | tail -n1 | sed 's/\"//g'`
deb_libqmi_proxy=`curl http://security.ubuntu.com/ubuntu/pool/main/libq/libqmi/ 2>/dev/null | grep -Eo '"libqmi-proxy.*amd64.deb"' | tail -n1 | sed 's/\"//g'`
deb_libqmi_utils=`curl http://security.ubuntu.com/ubuntu/pool/universe/libq/libqmi/ 2>/dev/null | grep -Eo '"libqmi-utils.*amd64.deb"' | tail -n1 | sed 's/\"//g'`
if [ ! -f $deb_minicom ]; then
    wget http://security.ubuntu.com/ubuntu/pool/universe/m/minicom/$deb_minicom
    dpkg -i $deb_minicom
fi
if [ ! -f $deb_libqmi_glib5 ]; then
    wget http://security.ubuntu.com/ubuntu/pool/main/libq/libqmi/$deb_libqmi_glib5
    dpkg -i $deb_libqmi_glib5
fi
if [ ! -f $deb_libqmi_proxy ]; then
    wget http://security.ubuntu.com/ubuntu/pool/main/libq/libqmi/$deb_libqmi_proxy
    dpkg -i $deb_libqmi_proxy
fi
if [ ! -f $deb_libqmi_utils ]; then
    wget http://security.ubuntu.com/ubuntu/pool/universe/libq/libqmi/$deb_libqmi_utils
    dpkg -i $deb_libqmi_utils
fi

# Install Modem Mode Switcher
if [ ! -f swi_setusbcomp.pl ]; then
    wget https://git.mork.no/wwan.git/plain/scripts/swi_setusbcomp.pl
    chmod +x ./swi_setusbcomp.pl
fi

devpath=`ls /dev | grep -E 'cdc-wdm|qcqmi'`

# Reset Modem
printf "${BLUE}---${NC}\n"
echo 'Reseting modem...'
./swi_setusbcomp.pl --usbreset --device="/dev/$devpath" &>/dev/null
sleep 3
# Modem Mode Switch to usbcomp=8 (DM   NMEA  AT    MBIM)
printf "${BLUE}---${NC}\n"
echo 'Running Modem Mode Switch to usbcomp=8 (DM   NMEA  AT    MBIM)'
./swi_setusbcomp.pl --usbcomp=8 --device="/dev/$devpath"

# Reset Modem
printf "${BLUE}---${NC}\n"
echo 'Reseting modem...'
./swi_setusbcomp.pl --usbreset --device="/dev/$devpath" &>/dev/null

deviceid=''
while [ -z $deviceid ]
do
    echo 'Waiting for modem to reboot...'
    sleep 3
    deviceid=`lsusb | grep -i -E '1199:9071|1199:9079|413C:81B6' | awk '{print $6}'`
done

sleep 5

ttyUSB=`dmesg | grep '.3: Qualcomm USB modem converter detected' -A1 | grep ttyUSB | sed 's/.*attached\ to\ //' | tail -1`
devpath=`ls /dev | grep -E 'cdc-wdm|qcqmi'`

# cat the serial port to monitor output and commands. cat will exit when AT!RESET kicks off.
sudo cat /dev/$ttyUSB 2>1 | tee -a modem.log &  

# Display current modem settings
printf "${BLUE}---${NC}\n"
echo 'Current modem settings:'
echo 'send AT
send ATE1
sleep 1
send ATI
sleep 1
send AT!ENTERCND=\"A710\"
sleep 1
send AT!IMPREF?
sleep 1
send AT!GOBIIMPREF?
sleep 1
send AT!USBCOMP?
sleep 1
send AT!USBCOMP=?
sleep 1
send AT!USBVID?
sleep 1
send AT!USBPID?
sleep 1
send AT!USBPRODUCT?
sleep 1
send AT!PRIID?
sleep 1
send AT!SELRAT?
sleep 1
send AT!BAND?
sleep 1
send AT!BAND=?
sleep 1
send AT!PCINFO?
sleep 1
send AT!IMAGE?
sleep 1
! pkill minicom
' > script.txt

sudo minicom -b 115200 -D /dev/$ttyUSB -S script.txt &>/dev/null
printf "${BLUE}---${NC}\n"
while [[ ! $REPLY =~ ^[Yy]$ ]]
do
    read -p '
Warning: This will overwrite all settings with generic EM7455/MC7455 Settings.
Are you sure you want to continue? (CTRL+C to exit) ' -n 1 -r
    if [[ $REPLY =~ ^[Nn]$ ]]
    then
        printf '\r\n'; break
    fi
done
printf '\r\n'

# Clear Previous PRI/FW Entries
echo 'send AT!IMAGE=0
sleep 1
send AT!IMAGE?
sleep 1
! pkill minicom
' > script.txt
sudo minicom -b 115200 -D /dev/$ttyUSB -S script.txt &>/dev/null

# Reset Modem
printf "${BLUE}---${NC}\n"
echo 'Reseting modem...'
./swi_setusbcomp.pl --usbreset --device="/dev/$devpath" &>/dev/null

zipsha512actual=`sha512sum SWI9X30C_02.30.01.01_Generic_002.045_000.zip |  awk '{print $1}'`
zipsha512expected='dad82310097c1ac66bb93da286c2e6f18b691cfea98df2756c8b044e5815087c9141325fc3e585c04f394c2a54e8d9b9bc2e5c5768cc7e0466d1321c1947cc8c'
if [ "$zipsha512actual" != "$zipsha512expected" ]; then
    printf "${BLUE}---${NC}\n"
    echo 'Download and unzip SWI9X30C_02.30.01.01_Generic_002.045_000 firmware...'
    curl -o SWI9X30C_02.30.01.01_Generic_002.045_000.zip -L https://source.sierrawireless.com/~/media/support_downloads/airprime/74xx/fw/02_30_01_01/7455/swi9x30c_02.30.01.01_generic_002.045_000.ashx
    unzip -o SWI9X30C_02.30.01.01_Generic_002.045_000.zip
fi

zipsha512actual=`sha512sum SWI9X30C_02.30.01.01_Generic_002.045_000.zip |  awk '{print $1}'`
if [ "$zipsha512actual" != "$zipsha512expected" ]; then 
    printf "${BLUE}---${NC}\n"
    printf "Download of ${BLUE}SWI9X30C_02.30.01.01_Generic_002.045_000.zip${NC} failed, exiting...\n"
    exit
fi

deviceid=`lsusb | grep -i -E '1199:9071|1199:9079|413C:81B6' | awk '{print $6}'`
while [ -z $deviceid ]
do
    echo 'Waiting for modem to reboot...'
    sleep 3
    deviceid=`lsusb | grep -i -E '1199:9071|1199:9079|413C:81B6' | awk '{print $6}'`
done
#Kill cat processes used for monitoring status, if it hasnt already exited
sudo pkill -9 cat &>/dev/null

printf "${BLUE}---${NC}\n"
# Flash SWI9X30C_02.30.01.01_Generic_002.045_000 onto Generic Sierra Modem
echo 'Flashing SWI9X30C_02.30.01.01_Generic_002.045_000 onto Generic Sierra Modem...'
sleep 5
qmi-firmware-update --update -d "$deviceid" SWI9X30C_02.30.01.01.cwe SWI9X30C_02.30.01.01_GENERIC_002.045_000.nvu
rc=$?
if [[ $rc != 0 ]]
then
    echo "Firmware Update failed, exiting..."
    exit $rc
fi

deviceid=''
while [ -z $deviceid ]
do
    echo 'Waiting for modem to reboot...'
    sleep 3
    deviceid=`lsusb | grep -i -E '1199:9071|1199:9079|413C:81B6' | awk '{print $6}'`
done

# cat the serial port to monitor output and commands. cat will exit when AT!RESET kicks off.
sudo cat /dev/$ttyUSB 2>1 | tee -a modem.log &  

# Set Generic Sierra Wireless VIDs/PIDs
if [[ $REPLY =~ ^[Yy]$ ]]
then
    echo 'send AT
send ATE1
sleep 1
send ATI
sleep 1
send AT!ENTERCND=\"A710\"
sleep 1
send AT!IMPREF=\"GENERIC\"
sleep 1
send AT!GOBIIMPREF=\"GENERIC\"
sleep 1
send AT!USBCOMP=1,1,0000100D
sleep 1
send AT!USBVID=1199
sleep 1
send AT!USBPID=9071,9070
sleep 1
send AT!USBPRODUCT=\"EM7455\"
sleep 1
send AT!PRIID=\"9904609\",\"002.030\",\"Generic-Laptop\"
sleep 1
send AT!SELRAT=06
sleep 1
send AT!BAND=00
sleep 1
send AT!IMAGE?
sleep 1
send AT!PCINFO?
sleep 1
send AT!RESET
! pkill minicom
' > script.txt
    sudo minicom -b 115200 -D /dev/$ttyUSB -S script.txt &>/dev/null
fi

#Done, restart ModemManager
systemctl enable ModemManager &>/dev/null
systemctl start ModemManager &>/dev/null

echo "Done!"

#Kill cat processes used for monitoring status, if it hasnt already exited
sudo pkill -9 cat &>/dev/null
